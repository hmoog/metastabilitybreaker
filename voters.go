package metastabilitybreaker

import (
	"fmt"

	"github.com/iotaledger/hive.go/events"
)

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// Vote represents a struct that contains the information about which Branch a certain Voter prefers.
type Vote struct {
	Issuer   VoterID
	BranchID BranchID
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

func (v *HonestVoter) SendVote() {
	v.network.VoteReceived.Trigger(&Vote{
		Issuer:   v.id,
		BranchID: v.consensus.FavoredBranch(),
	})
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

func (m *MinorityVoter) SendVote() {
	// do nothing, we have our own voting strategy based on the behavior of others
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

func (m *LowerHashVoter) SendVote() {
	// do nothing, we have our own voting strategy based on the behavior of others
}

func (m *LowerHashVoter) Type() string {
	return "LowerHashVoter"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voter ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Voter interface {
	ID() VoterID
	Type() string
	Network() *Network
	ApprovalWeightManager() *ApprovalWeightManager
	BranchManager() *BranchManager
	SendVote()
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
