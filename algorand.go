package main

import "sync/atomic"

type Algorand struct {
	pubkey  atomic.Value
	privkey PrivateKey

	chain *Blockchain
	peer  *Peer
}

func NewAlgorand() *Algorand {
	return &Algorand{
		chain: newBlockchain(),
		peer:  newPeer(),
	}
}

func (alg *Algorand) start() {

}

func (alg *Algorand) stop() {

}

func (alg *Algorand) round() uint64 {

}

func (alg *Algorand) publicKey() PublicKey {
	if pub := alg.pubkey.Load(); pub != nil {
		return pub.(PublicKey)
	}
	pubkey := alg.privkey.PublicKey()
	alg.pubkey.Store(pubkey)
	return pubkey
}

func (alg *Algorand) BA() {

}

func (alg *Algorand) committeeVote() {

}

func (alg *Algorand) reduction() {

}

func (alg *Algorand) binaryBA() {

}

func (alg *Algorand) countVotes() {

}

func (alg *Algorand) processMsg() {

}

func sortition() {

}

func verifySort() {

}
