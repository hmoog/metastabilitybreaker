package metastabilitybreaker

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/olekukonko/tablewriter"
)

// region Network //////////////////////////////////////////////////////////////////////////////////////////////////////

type Network struct {
	MetastabilityBreakingThreshold time.Duration
	Voters                         map[VoterID]Voter
	VoteReceived                   *events.Event
	WeightDistribution             *WeightDistribution
}

func NewNetwork(metastabilityBreakingThreshold time.Duration) *Network {
	return &Network{
		MetastabilityBreakingThreshold: metastabilityBreakingThreshold,
		Voters:                         make(map[VoterID]Voter),
		WeightDistribution:             NewWeightDistribution(),
		VoteReceived: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(*Vote))(params[0].(*Vote))
		}),
	}
}

func (n *Network) AddVoters(amount int, voterFactory VoterFactory, weightGenerator func(voterID VoterID) float64) {
	for i := 0; i < amount; i++ {
		voter := voterFactory(n)

		n.Voters[voter.ID()] = voter
		n.WeightDistribution.SetWeight(voter.ID(), weightGenerator(voter.ID()))

		n.VoteReceived.Attach(events.NewClosure(voter.OnVoteReceived))
	}
}

func (n *Network) ResolveConflicts(branchIDs ...BranchID) {
	for _, branchID := range branchIDs {
		n.VoteReceived.Trigger(&Vote{
			Issuer:   NewVoterID(),
			BranchID: branchID,
		})
	}

	go func() {
		for {
			for _, voter := range n.Voters {
				voter.SendVote()

				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (n *Network) ApprovalWeightByVoterType() (approvalWeightByVoterType map[string]map[BranchID]float64) {
	approvalWeightByVoterType = make(map[string]map[BranchID]float64)

	branchesWithKnownVoters := set.New()
	for _, voter := range n.Voters {
		honestVoter, ok := voter.(*HonestVoter)
		if !ok {
			continue
		}

		for voterID, branchID := range honestVoter.approvalWeightManager.LastStatements() {
			voter, voterExists := n.Voters[voterID]
			voterType := "<None>"
			var voterWeight float64
			if voterExists {
				branchesWithKnownVoters.Add(branchID)

				voterType = voter.Type()
				voterWeight = n.WeightDistribution.Weight(voter.ID())
			}

			if _, exists := approvalWeightByVoterType[voterType]; !exists {
				approvalWeightByVoterType[voterType] = make(map[BranchID]float64)
			}

			approvalWeightByVoterType[voterType][branchID] += voterWeight
		}

		break
	}

	for voterType, votesByBranch := range approvalWeightByVoterType {
		for branchID, _ := range votesByBranch {
			if voterType == "<None>" && branchesWithKnownVoters.Has(branchID) {
				delete(votesByBranch, branchID)
			}
		}
	}

	return approvalWeightByVoterType
}

func (n *Network) ConflictResolved() bool {
	expectedWeight := float64(0)
	for _, voter := range n.Voters {
		if voter.Type() != "HonestVoter" {
			continue
		}

		expectedWeight += n.WeightDistribution.Weight(voter.ID())
	}

	for _, weight := range n.ApprovalWeightByVoterType()["HonestVoter"] {
		if weight == expectedWeight {
			return true
		}
	}

	return false
}

func (n *Network) String() string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Voter", "BranchID", "Weight"})
	table.SetBorder(false)
	table.SetAutoFormatHeaders(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for voterType, votesByBranch := range n.ApprovalWeightByVoterType() {
		for branchID, amount := range votesByBranch {
			table.AppendBulk([][]string{
				{voterType, branchID.String(), fmt.Sprintf("%0.2f", amount)},
			})
		}
	}

	table.Render()

	return buf.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchManager ////////////////////////////////////////////////////////////////////////////////////////////////

type BranchManager struct {
	voter        Voter
	metadataByID map[BranchID]*BranchMetadata

	mutex sync.RWMutex
}

func NewBranchManager(voter Voter) *BranchManager {
	return &BranchManager{
		voter:        voter,
		metadataByID: make(map[BranchID]*BranchMetadata),
	}
}

func (b *BranchManager) BranchIDs() (branchIDs BranchIDs) {
	branchIDs = make(BranchIDs)
	for branchID := range b.metadataByID {
		branchIDs[branchID] = types.Void
	}

	return branchIDs
}

func (b *BranchManager) RegisterBranch(branchID BranchID) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, exists := b.metadataByID[branchID]; !exists {
		b.metadataByID[branchID] = &BranchMetadata{
			SolidificationTime: time.Now(),
		}
	}
}

func (b *BranchManager) Metadata(branchID BranchID) *BranchMetadata {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return b.metadataByID[branchID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchID /////////////////////////////////////////////////////////////////////////////////////////////////////

type BranchID int

var UndefinedBranchID BranchID

func NewBranchID(number int) BranchID {
	return BranchID(number)
}

func (b BranchID) String() string {
	return "BranchID(" + fmt.Sprintf("%d", b) + ")"
}

type BranchIDs map[BranchID]types.Empty

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

type BranchMetadata struct {
	SolidificationTime time.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApprovalWeightManager ////////////////////////////////////////////////////////////////////////////////////////

type ApprovalWeightManager struct {
	VoteProcessed *events.Event

	voter               Voter
	weights             map[BranchID]float64
	weightsMutex        sync.RWMutex
	lastStatements      map[VoterID]BranchID
	lastStatementsMutex sync.RWMutex
}

func NewApprovalWeightManager(voter Voter) *ApprovalWeightManager {
	return &ApprovalWeightManager{
		VoteProcessed: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(*Vote))(params[0].(*Vote))
		}),

		voter:          voter,
		weights:        make(map[BranchID]float64),
		lastStatements: make(map[VoterID]BranchID),
	}
}

func (a *ApprovalWeightManager) ProcessVote(vote *Vote) {
	a.lastStatementsMutex.Lock()
	defer a.lastStatementsMutex.Unlock()

	a.voter.BranchManager().RegisterBranch(vote.BranchID)

	lastBranchID, statementExists := a.lastStatements[vote.Issuer]
	if statementExists {
		if vote.BranchID == lastBranchID {
			return
		}

		a.updateWeight(lastBranchID, -a.voter.Network().WeightDistribution.Weight(vote.Issuer))
	}

	a.updateWeight(vote.BranchID, a.voter.Network().WeightDistribution.Weight(vote.Issuer))
	a.lastStatements[vote.Issuer] = vote.BranchID

	a.VoteProcessed.Trigger(vote)
}

func (a *ApprovalWeightManager) Weight(branchID BranchID) float64 {
	a.weightsMutex.RLock()
	defer a.weightsMutex.RUnlock()

	return a.weights[branchID]
}

func (a *ApprovalWeightManager) LastStatements() (lastStatements map[VoterID]BranchID) {
	a.lastStatementsMutex.RLock()
	defer a.lastStatementsMutex.RUnlock()

	lastStatements = make(map[VoterID]BranchID)
	for voterID, branchID := range a.lastStatements {
		lastStatements[voterID] = branchID
	}

	return lastStatements
}

func (a *ApprovalWeightManager) updateWeight(branchID BranchID, diff float64) {
	a.weightsMutex.Lock()
	defer a.weightsMutex.Unlock()

	a.weights[branchID] += diff
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WeightDistribution ///////////////////////////////////////////////////////////////////////////////////////////

type WeightDistribution struct {
	weights map[VoterID]float64
}

func NewWeightDistribution() *WeightDistribution {
	return &WeightDistribution{
		weights: make(map[VoterID]float64),
	}
}

func (w *WeightDistribution) SetWeight(voterID VoterID, weight float64) {
	w.weights[voterID] = weight
}

func (w *WeightDistribution) Weight(voterID VoterID) float64 {
	return w.weights[voterID]
}

func (w *WeightDistribution) String() string {
	weightDistribution := stringify.StructBuilder("WeightDistribution")
	for voterID, weight := range w.weights {
		weightDistribution.AddField(stringify.StructField(voterID.String(), fmt.Sprintf("%0.2f", weight)))
	}

	return weightDistribution.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
