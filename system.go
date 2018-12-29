package main

import "time"

const (
	totalTokenAmount           = 1000
	expectedProposerNum        = 26
	expectedCommitteeSize      = 2000
	threshold                  = 0.685
	expectedFinalCommitteeSize = 10000
	finalThreshold             = 0.74
	MAXSTEPS                   = 150
	lambdaPriority             = 5 * time.Second
	lambdaBlock                = 1 * time.Minute
	lambdaStep                 = 20 * time.Second
	lambdaStepvar              = 5 * time.Second

	committee = "committee"
)
