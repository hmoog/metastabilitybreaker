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
// ConfirmationThreshold is the threshold of collected statements at which we consider something confirmed
ConfirmationThreshold = 0.66

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
	metastabilityBreakingThreshold := timeScaling(branch1, branch2) * ConfirmationThreshold
	
	if deltaWeight(branch1, branch2) =< metastabilityBreakingThreshold {
		return branch1.hash < branch2.hash ? branch1 : branch2
	}
	
	return branch1.weight > branch2.weight ? branch1 : branch2
}
```

The longer the two branches stay pending, the larger the threshold gets at which we trigger the selection of the branch with the lower hash. It is important to note that this metastability breaker automatically disables itself once the ConfirmationThreshold is reached independently of how long the conflicting branches stay in the DAG.

If we now apply this algorithm to the two heaviest branches of a conflict set, then nodes should over time converge to the same branch. This is trivially true if the set only contains two branches `A` and `B`, but it even holds for larger sets:

Let's assume that the conflict set contains 3 branches `A`, `B` and `C` and let's also assume that `Hash(A) < Hash(B) < Hash(C)`.

There are now 3 different permutations for the candidates of the two heaviest branches: `A and B`,  `A and C`, `B and C`. Since `C` has the highest hash in any of the permutations, it will at some point no longer be able to gather any votes which reduces the set to the trivial case that has to decide between `A and B`.

This is true for any size of conflicts as there will always be a branch with the lowest hash, that will ultimately lose against its competitor when kept in a metastable state.

## Simulations

The code in this repository tests multiple different attack strategies:

- The MinorityVoter tries to keep the system undecided by always switching his opinion to the second-heaviest opinion.
- The LowerHashVoter introduces new lower hashes and tests the scenario where the conflict set is "open".

The simulation results show that the proposed mechanism is resilient against the MinorityVoter strategy even if they have a really large amount of influence in the system (e.g. 49% weight).

It is also resilient against the LowerHashVoter strategy but only if the attacker is not the node with the absolute most influence in the whole consensus. The reason for that is that in that case an attacker can continue to mine transactions with lower hashes, and gradually release them before any of the honest nodes is ever able to make a 2nd statement on one of his previous proposals.

While this attack is "theoretically" possible, it would be extremely hard to pull off in practice as it is nearly impossible to time the messages, so no two honest nodes will ever vote for the same branch and eventually exceed the weight of the attacker. The same attack would also be possible in any blockchain and the fact that we haven't seen something like this is a good indicator that it is an extremely unrealistic attack vector.

Nevertheless, it would also be "detectable" as the same attacker would give his weight to a new lower branch over and over again, and we could later add a behavioral aspect to the consensus weight that uses this kind of observation to modify the weight of the validators.

## Conclusion

The described metastability breaking mechanism is a very simple and straight forward extension of our vanilla consensus mechanism. The simulations show that it reliably breaks metastable states within 1-2 seconds.