package main

import (
	"bytes"
	"github.com/tinychain/algorand/common"
	"gonum.org/v1/gonum/stat/distuv"
	"math/big"
	"math/rand"
	"sync/atomic"
	"time"
)

type Algorand struct {
	pubkey  atomic.Value
	privkey *PrivateKey

	chain *Blockchain
	peer  *Peer

	// binomial distribution
	binomial *distuv.Binomial
}

func NewAlgorand() *Algorand {
	rand.Seed(time.Now().Unix())
	tokenOwned := rand.Float64()

	return &Algorand{
		chain: newBlockchain(),
		peer:  newPeer(),
		binomial: &distuv.Binomial{
			N: tokenOwned,
			P: float64(expectedProposerNum) / float64(totalTokenAmount),
		},
	}
}

func (alg *Algorand) start() {

}

func (alg *Algorand) stop() {

}

func (alg *Algorand) round() uint64 {

}

func (alg *Algorand) publicKey() *PublicKey {
	if pub := alg.pubkey.Load(); pub != nil {
		return pub.(*PublicKey)
	}
	pubkey := alg.privkey.PublicKey()
	alg.pubkey.Store(pubkey)
	return pubkey
}

func (alg *Algorand) BA() {

}

func (alg *Algorand) committeeVote(round uint64, step int) {
	role := alg.role(round, step)
	vrf, proof, j := alg.sortition(constructSeed())
	if j > 0 {
		//TODO: Gossip
	}
}

func (alg *Algorand) reduction() {

}

func (alg *Algorand) binaryBA() {

}

func (alg *Algorand) countVotes() {

}

func (alg *Algorand) processMsg() {

}

// role returns the role bytes from current round and step
func (alg *Algorand) role(round uint64, step int) []byte {
	return bytes.Join([][]byte{
		[]byte(committee),
		common.Uint2Bytes(round),
		common.Uint2Bytes(uint64(step)),
	}, nil)
}

func constructSeed(seed, role []byte) []byte {
	return bytes.Join([][]byte{seed, role}, nil)
}

// sortition runs cryptographic selection procedure and returns vrf,proof and amount of selected sub-users.
func (alg *Algorand) sortition(seed, role []byte) (vrf, proof []byte, selected int) {
	vrf, proof = alg.privkey.Evaluate(constructSeed(seed, role))

	selected = alg.subUsers(vrf)

	return
}

// verifySort verifies the vrf and returns the amount of selected sub-users.
func (alg *Algorand) verifySort(vrf, proof, seed, role []byte) int {
	if err := alg.publicKey().VerifyVRF(vrf, proof, constructSeed(seed, role)); err != nil {
		return 0
	}

	return alg.subUsers(vrf)
}

func (alg *Algorand) subUsers(vrf []byte) int {
	j := 0
	// hash / 2^hashlen
	hashBig := new(big.Float).SetInt(new(big.Int).SetBytes(vrf))
	denoBig := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength), big.NewInt(0)))
	t := hashBig.Quo(hashBig, denoBig)
	for {
		lower := big.NewFloat(alg.binomial.CDF(float64(j)))
		upper := big.NewFloat(alg.binomial.CDF(float64(j + 1)))
		if t.Cmp(lower) >= 0 && t.Cmp(upper) < 0 {
			break
		}
	}
	return j
}
