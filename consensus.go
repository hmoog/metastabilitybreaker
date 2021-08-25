package metastabilitybreaker

import (
	"math"
	"time"
)

const (
	confirmationThreshold = 0.66
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	voter Voter
	timeOffset time.Duration
}

func NewConsensus(voter Voter) *Consensus {
	return &Consensus{
		voter: voter,
	}
}

func (c *Consensus) CompetingBranches() (largestBranch, secondLargestBranch BranchID) {
	var largestBranchWeight, secondLargestBranchWeight float64
	for branchID := range c.voter.BranchManager().BranchIDs() {
		branchWeight := c.voter.ApprovalWeightManager().Weight(branchID)
		if branchWeight >= largestBranchWeight {
			secondLargestBranch = largestBranch
			secondLargestBranchWeight = largestBranchWeight

			largestBranchWeight = branchWeight
			largestBranch = branchID
		} else if branchWeight >= secondLargestBranchWeight {
			secondLargestBranch = branchID
			secondLargestBranchWeight = branchWeight
		}
	}

	return
}

func (c *Consensus) FavoredBranch() BranchID {
	heaviestBranch, secondHeaviestBranch := c.CompetingBranches()
	if heaviestBranch == UndefinedBranchID || secondHeaviestBranch == UndefinedBranchID {
		return heaviestBranch
	}

	if c.voter.Network().MetastabilityBreakingThreshold != 0 && c.deltaWeight(heaviestBranch, secondHeaviestBranch) <= c.timeScaling(heaviestBranch, secondHeaviestBranch)*confirmationThreshold {
		if heaviestBranch < secondHeaviestBranch {
			return heaviestBranch
		}

		return secondHeaviestBranch
	}

	if c.voter.ApprovalWeightManager().Weight(heaviestBranch) > c.voter.ApprovalWeightManager().Weight(secondHeaviestBranch) {
		return heaviestBranch
	}

	return secondHeaviestBranch
}

func (c *Consensus) deltaWeight(branch1ID, branch2ID BranchID) float64 {
	return math.Abs(c.voter.ApprovalWeightManager().Weight(branch1ID) - c.voter.ApprovalWeightManager().Weight(branch2ID))
}

func (c *Consensus) pendingTime(branch1ID, branch2ID BranchID) time.Duration {
	branch1SolidificationTime := c.voter.BranchManager().Metadata(branch1ID).SolidificationTime
	branch2SolidificationTime := c.voter.BranchManager().Metadata(branch2ID).SolidificationTime

	if branch1SolidificationTime.After(branch2SolidificationTime) {
		return time.Now().Add(c.timeOffset).Sub(branch1SolidificationTime)
	}

	return time.Now().Add(c.timeOffset).Sub(branch2SolidificationTime)
}

func (c *Consensus) timeScaling(branch1ID, branch2ID BranchID) float64 {
	return math.Min(float64(c.pendingTime(branch1ID, branch2ID).Nanoseconds())/float64(c.voter.Network().MetastabilityBreakingThreshold.Nanoseconds()), 1)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
