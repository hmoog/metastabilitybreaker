package metastabilitybreaker

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/events"
)

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// Vote represents a struct that contains the information about which Branch a certain Voter prefers.
type Vote struct {
	Issuer   VoterID
	BranchID BranchID
}

func (v *Vote) String() string {
	return v.Issuer.String() + " votes for " + v.BranchID.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region HonestVoter //////////////////////////////////////////////////////////////////////////////////////////////////

// HonestVoter implements the behavior of the honest actors, that always vote for the favored Branch.
type HonestVoter struct {
	id                    VoterID
	branchManager         *BranchManager
	approvalWeightManager *ApprovalWeightManager
	consensus             *Consensus
	network               *Network
}

// NewHonestVoter returns a new HonestVoter instance.
func NewHonestVoter(network *Network) (voter Voter) {
	honestVoter := &HonestVoter{
		id:      NewVoterID(),
		network: network,
	}
	honestVoter.branchManager = NewBranchManager(honestVoter)
	honestVoter.approvalWeightManager = NewApprovalWeightManager(honestVoter)
	honestVoter.consensus = NewConsensus(honestVoter)

	return honestVoter
}

// ID returns the identifier of the Voter.
func (v *HonestVoter) ID() VoterID {
	return v.id
}

// Type returns a string that describes the type of the Voter.
func (v *HonestVoter) Type() string {
	return "HonestVoter"
}

// ApprovalWeightManager returns the ApprovalWeightManager instance that is used by the Voter.
func (v *HonestVoter) ApprovalWeightManager() *ApprovalWeightManager {
	return v.approvalWeightManager
}

// BranchManager returns the BranchManager instance that is used by the Voter.
func (v *HonestVoter) BranchManager() *BranchManager {
	return v.branchManager
}

// Network returns the Network instance that the Voter is connected to.
func (v *HonestVoter) Network() *Network {
	return v.network
}

func (v *HonestVoter) OnVoteReceived(vote *Vote) {
	v.approvalWeightManager.ProcessVote(vote)
}

func (v *HonestVoter) SendVote() (opinionChanged bool) {
	favoredBranch := v.consensus.FavoredBranch()
	if favoredBranch == v.approvalWeightManager.lastStatements[v.id] {
		return false
	}

	v.network.VoteReceived.Trigger(&Vote{
		Issuer:   v.id,
		BranchID: favoredBranch,
	})

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MinorityVoter ////////////////////////////////////////////////////////////////////////////////////////////////

type MinorityVoter struct {
	*HonestVoter
}

func NewMinorityVoter(network *Network) (voter Voter) {
	minorityVoter := &MinorityVoter{
		HonestVoter: NewHonestVoter(network).(*HonestVoter),
	}

	minorityVoter.ApprovalWeightManager().VoteProcessed.Attach(events.NewClosure(minorityVoter.VoteProcessed))

	return minorityVoter
}

func (m *MinorityVoter) Type() string {
	return "MinorityVoter"
}

func (m *MinorityVoter) VoteProcessed(vote *Vote) {
	if issuer, issuerExists := m.Network().Voters[vote.Issuer]; issuerExists && issuer.Type() == "HonestVoter" {
		_, secondLargestBranch := m.HonestVoter.consensus.CompetingBranches()

		go func() {
			m.network.VoteReceived.Trigger(&Vote{
				Issuer:   m.id,
				BranchID: secondLargestBranch,
			})
		}()
	}
}

func (m *MinorityVoter) SendVote() (opinionChanged bool) {
	// do nothing, we have our own voting strategy based on the behavior of others
	return false
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LowerHashVoter ///////////////////////////////////////////////////////////////////////////////////////////////

type LowerHashVoter struct {
	*HonestVoter

	lowestBranch BranchID
}

func NewLowerHashVoter(network *Network) (voter Voter) {
	lowerHashVoter := &LowerHashVoter{
		HonestVoter: NewHonestVoter(network).(*HonestVoter),
	}

	lowerHashVoter.ApprovalWeightManager().VoteProcessed.Attach(events.NewClosure(lowerHashVoter.VoteProcessed))

	return lowerHashVoter
}

func (m *LowerHashVoter) VoteProcessed(vote *Vote) {
	if issuer, issuerExists := m.Network().Voters[vote.Issuer]; issuerExists && issuer.Type() == "HonestVoter" {
		lowerBranch := vote.BranchID - 1

		go func() {
			m.network.VoteReceived.Trigger(&Vote{
				Issuer:   m.id,
				BranchID: lowerBranch,
			})
		}()
	}
}

func (m *LowerHashVoter) SendVote() (opinionChanged bool) {
	// do nothing, we have our own voting strategy based on the behavior of others
	return false
}

func (m *LowerHashVoter) Type() string {
	return "LowerHashVoter"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SlowMinorityVoter ///////////////////////////////////////////////////////////////////////////////////////////////

type SlowMinorityVoter struct {
	*HonestVoter

	lowestBranch BranchID
}

func NewSlowMinorityVoter(network *Network) (voter Voter) {
	slowMinorityVoter := &SlowMinorityVoter{
		HonestVoter: NewHonestVoter(network).(*HonestVoter),
	}

	network.BeforeNextVote.Attach(events.NewClosure(slowMinorityVoter.BeforeNextVote))
	network.VoteReceived.AttachAfter(events.NewClosure(func(vote *Vote) {
		issuer, exists := network.Voters[vote.Issuer]
		if !exists {
			return
		}

		fmt.Println("==", issuer.Type(), issuer.ID(), "votes for", vote.BranchID)
		fmt.Println()
		fmt.Println(slowMinorityVoter.approvalWeightManager.StringBranchWeights())
		fmt.Printf("lowerHashThreshold = %0.2f\n", slowMinorityVoter.consensus.timeScaling(BranchID(1), BranchID(2)) * confirmationThreshold)
		fmt.Println()
	}))

	return slowMinorityVoter
}

func (m *SlowMinorityVoter) BeforeNextVote(voter Voter) {
	if voter.Type() != "HonestVoter" {
		return
	}

	predictedBranch := m.consensus.FavoredBranch()
	var minorityBranch BranchID
	if largestBranch, secondLargestBranch := m.consensus.CompetingBranches(); largestBranch == predictedBranch {
		minorityBranch = secondLargestBranch
	} else {
		minorityBranch = largestBranch
	}

	reverseSimulatedVote := m.simulateVote(voter.ID(), predictedBranch)
	reverseSimulatedAttackerVote := m.simulateVote(m.ID(), minorityBranch)
	m.consensus.timeOffset = 100 * time.Millisecond
	predictedBranchAfterAttack := m.consensus.FavoredBranch()
	m.consensus.timeOffset = 0 * time.Millisecond
	reverseSimulatedVote()
	reverseSimulatedAttackerVote()

	if predictedBranchAfterAttack != minorityBranch && m.approvalWeightManager.lastStatements[m.id] != minorityBranch {
		m.network.VoteReceived.Trigger(&Vote{
			Issuer:   m.id,
			BranchID: minorityBranch,
		})
	}
}

func (m *SlowMinorityVoter) simulateVote(voterID VoterID, branchID BranchID) (reverser func()) {
	m.BranchManager().RegisterBranch(branchID)

	lastBranchID, exists := m.approvalWeightManager.LastStatements()[voterID]
	if !exists {
		m.approvalWeightManager.updateWeight(branchID, m.network.WeightDistribution.Weight(voterID))

		m.approvalWeightManager.lastStatements[voterID] = branchID

		return func() {
			delete(m.approvalWeightManager.lastStatements, voterID)

			m.approvalWeightManager.updateWeight(branchID, -m.network.WeightDistribution.Weight(voterID))
		}
	}

	m.approvalWeightManager.updateWeight(lastBranchID, -m.network.WeightDistribution.Weight(voterID))
	m.approvalWeightManager.updateWeight(branchID, m.network.WeightDistribution.Weight(voterID))

	m.approvalWeightManager.lastStatements[voterID] = branchID

	return func() {
		m.approvalWeightManager.lastStatements[voterID] = lastBranchID

		m.approvalWeightManager.updateWeight(lastBranchID, m.network.WeightDistribution.Weight(voterID))
		m.approvalWeightManager.updateWeight(branchID, -m.network.WeightDistribution.Weight(voterID))
	}
}

func (m *SlowMinorityVoter) SendVote() (opinionChanged bool) {
	// do nothing, we have our own voting strategy based on the behavior of others
	return false
}

func (m *SlowMinorityVoter) Type() string {
	return "SlowMinorityVoter"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voter ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Voter interface {
	ID() VoterID
	Type() string
	Network() *Network
	ApprovalWeightManager() *ApprovalWeightManager
	BranchManager() *BranchManager
	SendVote() (opinionChanged bool)
	OnVoteReceived(vote *Vote)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region VoterID //////////////////////////////////////////////////////////////////////////////////////////////////////

type VoterID int

var voterID VoterID

func NewVoterID() VoterID {
	voterID++

	return voterID
}

func (v VoterID) String() string {
	return "VoterID(" + fmt.Sprintf("%d", v) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region VoterFactory /////////////////////////////////////////////////////////////////////////////////////////////////

// VoterFactory represents a generic interface for the constructors of different types of Voters.
type VoterFactory func(network *Network) Voter

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
