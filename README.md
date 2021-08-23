# Metastabilitybreaker
This package implements a testing framework for different metastability breaking mechanisms for IOTAs multiverse consensus, and it uses this framework to propose and test a new and simpler metastability breaking mechanism.

## Motivation

We currently plan to use FPC (which is based on an external source of randomness) to break metastable states.

While it is easy to understand that this will work (and it can even be mathematically proven), I think that it is unnecessarily complex and it also prevents us from using a different sybil protection mechanism down the line (because the randomness is provided by a deterministic protocol with a fixed size committee).

# Deterministic metastability breaker

The basic idea behind the new approach is to use the deterministic nature of computer programs to break metastable states. Determinism in this context means that if you provide the same input to an algorithm, then it will produce the same output.

Following this line of thought, it should be possible to design an algorithm that produces approximately the same result when working on approximately the same input.

The tangle is a data structure that allows nodes to converge to a similar perception. While this perception will never be exactly the same, it is still reasonable to assume that nodes have more or less the same perception of the existing branches and their approval weight.

Our vanilla consensus algorithm works in the following way:

When choosing between two conflicting branches, we choose the one with the higher approval weight. If both branches have exactly the same weight, then we choose the one with the lower hash.

In code it would look like this:

```go
FavoredBranch(branch1, branch2) {
	if deltaWeight(branch1, branch2) == 0 {
		return branch1.hash < branch2.hash ? branch1 : branch2
	}
	
	return branch1.weight > branch2.weight ? branch1 : branch2
}
```

When written in this "guard programming" style, it becomes very obvious, that the "lower hash rule" is our tie-breaker for metastable states with the exact same weight.

Instead of only choosing the branch with the lower hash when their weight is exactly the same, I now propose to gradually (over time) increase the allowed deltaWeight for this to trigger. The modified code would look like this:

```go
// MetastabilityBreakingThreshold defines the time until when the metastability breaking mechanism will unfold its full power. 
MetastabilityBreakingThreshold = 5 s

// PendingTime returns the amount of time that the two Branches have been conflicting already (now - arrival time of the conflict).
PendingTime(branch1, branch2) {
	return now() - max(branch1.solidificationTime, branch2.solidificationTime)
}

// TimeScaling returns a number between 0 (the later branch just arrived) and 1 (the later branch arrived more than <MetastabilityBreakingThreshold> seconds ago)
TimeScaling(branch1, branch2) {
	return min(PendingTime(branch1, branch2) / MetastabilityBreakingThreshold, 1)
}

FavoredBranch(branch1, branch2) {
	metastabilityBreakingThreshold := timeScaling(branch1, branch2) * confirmationThreshold
	
	if deltaWeight(branch1, branch2) =< metastabilityBreakingThreshold {
		return branch1.hash < branch2.hash ? branch1 : branch2
	}
	
	return branch1.weight > branch2.weight ? branch1 : branch2
}
```

The longer the two branches stay pending, the larger the threshold gets at which we trigger the selection of the branch with the lower hash and the more likely, nodes are to choose the corresponding branch.