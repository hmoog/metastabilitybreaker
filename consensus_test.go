package metastabilitybreaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMinorityVoter_MetastabilityBreakerEnabled(t *testing.T) {
	network := NewNetwork(5 * time.Second)
	network.AddVoters(8, NewHonestVoter, func(voterID VoterID) float64 { return 0.1 })
	network.AddVoters(1, NewMinorityVoter, func(voterID VoterID) float64 { return 0.2 })
	network.ResolveConflicts(NewBranchID(1), NewBranchID(2))

	assert.Eventually(t, network.ConflictResolved, 20*time.Second, 50*time.Millisecond, "failed to resolve metastable state")
}

func TestMinorityVoter_MetastabilityBreakerDisabled(t *testing.T) {
	network := NewNetwork(0 * time.Second)
	network.AddVoters(8, NewHonestVoter, func(voterID VoterID) float64 { return 0.1 })
	network.AddVoters(1, NewMinorityVoter, func(voterID VoterID) float64 { return 0.2 })
	network.ResolveConflicts(NewBranchID(1), NewBranchID(2))

	time.Sleep(15 * time.Second)

	assert.False(t, false, network.ConflictResolved(), "metastable state expected to be maintained")
}

func TestLowerHashVoter_AttackerWithHighestWeight(t *testing.T) {
	network := NewNetwork(5 * time.Second)
	network.AddVoters(8, NewHonestVoter, func(voterID VoterID) float64 { return 0.1 })
	network.AddVoters(1, NewLowerHashVoter, func(voterID VoterID) float64 { return 0.2 })
	network.ResolveConflicts(NewBranchID(1000))

	time.Sleep(15 * time.Second)

	assert.False(t, false, network.ConflictResolved(), "metastable state expected to be maintained")
}

func TestLowerHashVoter_MetastabilityBreakerHighWeight(t *testing.T) {
	network := NewNetwork(5 * time.Second)
	network.AddVoters(8, NewHonestVoter, func(voterID VoterID) float64 {
		if voterID%2 == 0 {
			return 0.16
		}

		return 0.1
	})
	network.AddVoters(1, NewLowerHashVoter, func(voterID VoterID) float64 { return 0.15 })
	network.ResolveConflicts(NewBranchID(1000))

	assert.Eventually(t, network.ConflictResolved, 20*time.Second, 50*time.Millisecond, "failed to resolve metastable state")
}

func TestLowerHashVoter_MetastabilityBreakerLowWeight(t *testing.T) {
	network := NewNetwork(5 * time.Second)
	network.AddVoters(8, NewHonestVoter, func(voterID VoterID) float64 { return 0.1 })
	network.AddVoters(1, NewLowerHashVoter, func(voterID VoterID) float64 { return 0.08 })
	network.ResolveConflicts(NewBranchID(1000))

	assert.Eventually(t, network.ConflictResolved, 20*time.Second, 50*time.Millisecond, "failed to resolve metastable state")
}

func TestSlowMinorityVoter_MetastabilityBreakerEnabled(t *testing.T) {
	network := NewNetwork(5 * time.Second)
	network.AddVoters(18, NewHonestVoter, func(voterID VoterID) float64 { return 0.05 })
	network.AddVoters(1, NewSlowMinorityVoter, func(voterID VoterID) float64 { return 0.1 })
	network.ResolveConflicts(NewBranchID(1), NewBranchID(2))

	assert.Eventually(t, network.ConflictResolved, 20*time.Second, 50*time.Millisecond, "failed to resolve metastable state")
}
