package main

import (
	"encoding/json"
	"github.com/tinychain/algorand/common"
)

type VoteMessage struct {
	Signature  []byte      `json:"publicKey"`
	Round      uint64      `json:"round"`
	Step       int         `json:"step"`
	VRF        []byte      `json:"vrf"`
	Proof      []byte      `json:"proof"`
	ParentHash common.Hash `json:"parentHash"`
	Hash       common.Hash `json:"hash"`
}

func (v *VoteMessage) Serialize() ([]byte, error) {
	return json.Marshal(v)
}

func (v *VoteMessage) Deserialize(data []byte) error {
	return json.Unmarshal(data, v)
}

func (v *VoteMessage) VerifySign() error {
	return nil
}
