package main

import (
	"bytes"
	"encoding/json"
	"github.com/tinychain/algorand/common"
)

const (
	// message type
	VOTE = iota
	BLOCK_PROPOSAL
)

type BasicMessage struct {
	Type int    `json:"type"`
	Data []byte `json:"data"`
}

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

func (v *VoteMessage) VerifySign(pubkey *PublicKey) error {
	data := bytes.Join([][]byte{
		common.Uint2Bytes(v.Round),
		common.Uint2Bytes(uint64(v.Step)),
		v.VRF,
		v.Proof,
		v.ParentHash.Bytes(),
		v.Hash.Bytes(),
	}, nil)
	return pubkey.VerifySign(data, v.Signature)
}

func (v *VoteMessage) Sign(priv *PrivateKey) ([]byte, error) {
	data := bytes.Join([][]byte{
		common.Uint2Bytes(v.Round),
		common.Uint2Bytes(uint64(v.Step)),
		v.VRF,
		v.Proof,
		v.ParentHash.Bytes(),
		v.Hash.Bytes(),
	}, nil)
	sign, err := priv.Sign(data)
	if err != nil {
		return nil, err
	}
	v.Signature = sign
	return sign, nil
}

func (v *VoteMessage) RecoverPubkey() *PublicKey {
	return recoverPubkey(v.Signature)
}
