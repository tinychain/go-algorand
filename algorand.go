package main

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/tinychain/algorand/common"
	"gonum.org/v1/gonum/stat/distuv"
	"math/big"
	"math/rand"
	"sync/atomic"
	"time"
)

var (
	errCountVotesTimeout = errors.New("count votes timeout")
)

type Algorand struct {
	pubkey  atomic.Value
	privkey *PrivateKey

	chain *Blockchain
	peer  *Peer

	// context
	weight uint64 // weight is the total amount of tokens an account owns.
}

func NewAlgorand() *Algorand {
	rand.Seed(time.Now().Unix())
	tokenOwned := rand.Uint64()

	return &Algorand{
		chain:  newBlockchain(),
		peer:   newPeer(),
		weight: tokenOwned,
	}
}

func (alg *Algorand) start() {

}

func (alg *Algorand) stop() {

}

// round returns the current round number.
func (alg *Algorand) round() uint64 {
	now := time.Now().Unix()
	genesisTime := alg.chain.genesis.Time
	return uint64((genesisTime-now)/int64(timeoutPerRound) + 1)
}

func (alg *Algorand) seed() []byte {
	last := alg.chain.last
	return last.Hash().Bytes()
}

func (alg *Algorand) publicKey() *PublicKey {
	if pub := alg.pubkey.Load(); pub != nil {
		return pub.(*PublicKey)
	}
	pubkey := alg.privkey.PublicKey()
	alg.pubkey.Store(pubkey)
	return pubkey
}

func (alg *Algorand) run() {
	// propose block
}

// sortition runs cryptographic selection procedure and returns vrf,proof and amount of selected sub-users.
func (alg *Algorand) sortition(seed, role []byte, expectedNum int, weight uint64) (vrf, proof []byte, selected int) {
	vrf, proof = alg.privkey.Evaluate(constructSeed(seed, role))
	selected = subUsers(expectedNum, weight, vrf)
	return
}

// verifySort verifies the vrf and returns the amount of selected sub-users.
func (alg *Algorand) verifySort(vrf, proof, seed, role []byte, expectedNum int) int {
	if err := alg.publicKey().VerifyVRF(vrf, proof, constructSeed(seed, role)); err != nil {
		return 0
	}

	return subUsers(expectedNum, alg.weight, vrf)
}

// committeeVote votes for `value`.
func (alg *Algorand) committeeVote(round uint64, step int, expectedNum int, hash common.Hash) error {
	role := role(round, step)
	vrf, proof, j := alg.sortition(alg.seed(), role, expectedNum, alg.weight)

	if j > 0 {
		// Gossip vote message
		voteMsg := &VoteMessage{
			Round:      round,
			Step:       step,
			VRF:        vrf,
			Proof:      proof,
			ParentHash: alg.chain.last.Hash(),
			Hash:       hash,
		}
		_, err := voteMsg.Sign(alg.privkey)
		if err != nil {
			return err
		}
		data, err := voteMsg.Serialize()
		if err != nil {
			return err
		}
		alg.peer.gossip(VOTE, data)
	}
	return nil
}

// BA runs BA* for the next round, with a proposed block.
func (alg *Algorand) BA(round uint64, block *Block) {
	hash := alg.reduction(round, block.Hash())
	hash = alg.binaryBA(round, hash)
	r, _ := alg.countVotes(round, FINAL, finalThreshold, expectedFinalCommitteeMembers, lamdaStep)
	if r == hash {

	} else {

	}
}

// The two-step reduction.
func (alg *Algorand) reduction(round uint64, hash common.Hash) common.Hash {
	// step 1: gossip the block hash
	alg.committeeVote(round, REDUCTION_ONE, expectedCommitteeMembers, hash)

	// other users might still be waiting for block proposals,
	// so set timeout for λblock + λstep
	hash1, err := alg.countVotes(round, REDUCTION_ONE, thresholdOfBAStep, expectedCommitteeMembers, lamdaBlock+lamdaStep)

	// step 2: re-gossip the popular block hash
	empty := emptyHash(round, alg.chain.last.Hash())

	if err == errCountVotesTimeout {
		alg.committeeVote(round, REDUCTION_TWO, expectedCommitteeMembers, empty)
	} else {
		alg.committeeVote(round, REDUCTION_TWO, expectedCommitteeMembers, hash1)
	}

	hash2, err := alg.countVotes(round, REDUCTION_TWO, thresholdOfBAStep, expectedCommitteeMembers, lamdaStep)
	if err == errCountVotesTimeout {
		return empty
	}
	return hash2
}

// binaryBA executes until consensus is reached on either the given `hash` or `empty_hash`.
func (alg *Algorand) binaryBA(round uint64, hash common.Hash) common.Hash {
	var (
		step = 1
		r    = hash
		err  error
	)
	empty := emptyHash(round, alg.chain.last.Hash())
	for step < MAXSTEPS {
		alg.committeeVote(round, step, expectedCommitteeMembers, r)
		r, err = alg.countVotes(round, step, thresholdOfBAStep, expectedCommitteeMembers, lamdaStep)
		if err == errCountVotesTimeout {
			r = hash
		} else if r != empty {
			for s := step + 1; s <= step+3; s++ {
				alg.committeeVote(round, s, expectedCommitteeMembers, r)
			}
			if step == 1 {
				alg.committeeVote(round, FINAL, expectedFinalCommitteeMembers, r)
			}
			return r
		}
		step++

		alg.committeeVote(round, step, expectedCommitteeMembers, r)
		r, err = alg.countVotes(round, step, thresholdOfBAStep, expectedCommitteeMembers, lamdaStep)
		if err == errCountVotesTimeout {
			r = empty
		} else if r == empty {
			for s := step + 1; s <= step+3; s++ {
				alg.committeeVote(round, s, expectedCommitteeMembers, r)
			}
			return r
		}
		step++

		alg.committeeVote(round, step, expectedCommitteeMembers, r)
		r, err = alg.countVotes(round, step, thresholdOfBAStep, expectedCommitteeMembers, lamdaStep)
		if err == errCountVotesTimeout {
			if alg.commonCoin(round, step, expectedCommitteeMembers) == 0 {
				r = hash
			} else {
				r = empty
			}
		}
	}

	// hang forever
	<-make(chan struct{})
	return common.Hash{}
}

// countVotes counts votes for round and step.
func (alg *Algorand) countVotes(round uint64, step int, threshold float64, expectedNum int, timeout time.Duration) (common.Hash, error) {
	expired := time.NewTimer(timeout)
	counts := make(map[common.Hash]int)
	voters := make(map[PublicKey]struct{})
	it := alg.peer.iterator(constructMsgKey(round, step))
	for {
		msg := it.next()
		if msg == nil {
			select {
			case <-expired.C:
				// timeout
				return common.Hash{}, errCountVotesTimeout
			default:
			}
		} else {
			voteMsg := msg.(*VoteMessage)
			votes, hash, _ := alg.processMsg(msg.(*VoteMessage), expectedNum)
			pubkey := voteMsg.RecoverPubkey()
			if _, exist := voters[*pubkey]; exist || votes == 0 {
				continue
			}
			voters[*pubkey] = struct{}{}
			counts[hash] += votes
			// if we got enough votes, then output the target hash
			if uint64(counts[hash]) >= uint64(float64(expectedNum)*threshold) {
				return hash, nil
			}
		}

	}
}

// processMsg validates incoming vote message.
func (alg *Algorand) processMsg(message *VoteMessage, expectedNum int) (votes int, hash common.Hash, vrf []byte) {
	pubkey := message.RecoverPubkey()
	if err := message.VerifySign(pubkey); err != nil {
		return 0, common.Hash{}, nil
	}

	// discard messages that do not extend this chain
	prevHash := message.ParentHash
	if prevHash != alg.chain.last.Hash() {
		return 0, common.Hash{}, nil
	}

	votes = alg.verifySort(message.VRF, message.Proof, alg.seed(), role(message.Round, message.Step), expectedNum)
	hash = message.Hash
	vrf = message.VRF
	return
}

// commonCoin computes a coin common to all users.
// It is a procedure to help Algorand recover if an adversary sends faulty messages to the network and prevents the network from coming to consensus.
func (alg *Algorand) commonCoin(round uint64, step int, expectedNum int) int64 {
	minhash := new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength), big.NewInt(0))
	msgList := alg.peer.getIncomingMsgs(constructMsgKey(round, step))
	for _, m := range msgList {
		msg := m.(*VoteMessage)
		votes, _, vrf := alg.processMsg(msg, expectedNum)
		for j := 1; j < votes; j++ {
			h := new(big.Int).SetBytes(common.Sha256(bytes.Join([][]byte{vrf, common.Uint2Bytes(uint64(j))}, nil)).Bytes())
			if h.Cmp(minhash) < 0 {
				minhash = h
			}
		}
	}
	return minhash.Mod(minhash, big.NewInt(2)).Int64()
}

// role returns the role bytes from current round and step
func role(round uint64, step int) []byte {
	return bytes.Join([][]byte{
		[]byte(committee),
		common.Uint2Bytes(round),
		common.Uint2Bytes(uint64(step)),
	}, nil)
}

// priority returns the priority of block proposal.
// The parameter `vrf` is the hash output of VRF, and `index` is the sub-users' index.
func priority(vrf []byte, index int) uint32 {
	data := common.Sha256(bytes.Join([][]byte{vrf, common.Uint2Bytes(uint64(index))}, nil))
	return uint32(new(big.Int).SetBytes(data.Bytes()).Uint64())
}

// subUsers return the selected amount of sub-users determined from the mathematics protocol.
func subUsers(expectedNum int, weight uint64, vrf []byte) int {
	binomial := &distuv.Binomial{
		N: float64(weight),
		P: float64(expectedNum) / float64(totalTokenAmount),
	}
	j := 0
	// hash / 2^hashlen
	hashBig := new(big.Float).SetInt(new(big.Int).SetBytes(vrf))
	denoBig := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength), big.NewInt(0)))
	t := hashBig.Quo(hashBig, denoBig)
	for {
		lower := big.NewFloat(binomial.CDF(float64(j)))
		upper := big.NewFloat(binomial.CDF(float64(j + 1)))
		if t.Cmp(lower) >= 0 && t.Cmp(upper) < 0 {
			break
		}
	}
	return j
}

func constructSeed(seed, role []byte) []byte {
	return bytes.Join([][]byte{seed, role}, nil)
}

func emptyHash(round uint64, prev common.Hash) common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		common.Uint2Bytes(round),
		prev.Bytes(),
	}, nil))
}
