package main

import "time"

const (
	// Algorand system parameters
	totalTokenAmount = 10000
	node             = 10
	tokenPerNode     = totalTokenAmount / node

	expectedBlockProposers        = 26
	expectedCommitteeMembers      = 2000
	thresholdOfBAStep             = 0.685
	expectedFinalCommitteeMembers = 10000
	finalThreshold                = 0.74
	MAXSTEPS                      = 150

	// timeout param
	lamdaPriority = 5 * time.Second  // time to gossip sortition proofs.
	lamdaBlock    = 1 * time.Minute  // timeout for receiving a block.
	lamdaStep     = 20 * time.Second // timeout for BA* step.
	lamdaStepvar  = 5 * time.Second  // estimate of BA* completion time variance.

	// interval
	R                   = 1000          // seed refresh interval (# of rounds)
	forkResolveInterval = 1 * time.Hour // fork resolve interval time

	// helper const var
	committee   = "committee"
	proposer    = "proposer"
	forkResolve = "forkresolve"

	// step
	PROPOSE       = 1000
	REDUCTION_ONE = 1001
	REDUCTION_TWO = 1002
	FINAL         = 1003

	FINAL_CONSENSUS     = 0
	TENTATIVE_CONSENSUS = 1
)
