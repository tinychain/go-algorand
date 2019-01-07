package main

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"github.com/tinychain/algorand/common"
	"gonum.org/v1/gonum/stat/distuv"
	"math/big"
	"sync/atomic"
	"time"
)

var (
	errCountVotesTimeout = errors.New("count votes timeout")
)

type PID int

type Algorand struct {
	id      PID
	pubkey  atomic.Value
	privkey *PrivateKey

	chain  *Blockchain
	peer   *Peer
	quitCh chan struct{}
}

func NewAlgorand(id PID) *Algorand {
	alg := &Algorand{
		id:    id,
		chain: newBlockchain(),
	}
	alg.peer = newPeer(alg)
	return alg
}

func (alg *Algorand) start() {
	alg.quitCh = make(chan struct{})
	go alg.run()
}

func (alg *Algorand) stop() {
	close(alg.quitCh)
}

// round returns the latest round number.
func (alg *Algorand) round() uint64 {
	return alg.lastBlock().Round
}

func (alg *Algorand) lastBlock() *Block {
	return alg.chain.last
}

// weight returns the weight of the given address.
func (alg *Algorand) weight(address common.Address) uint64 {
	return tokenPerNode
}

// tokenOwn returns the token amount (weight) owned by self node.
func (alg *Algorand) tokenOwn() uint64 {
	return alg.weight(alg.Address())
}

// seed returns the vrf-based seed of block r.
func (alg *Algorand) vrfSeed(round uint64) (seed, proof []byte) {
	if round == 0 {
		return alg.chain.genesis.Seed, nil
	}
	lastBlock := alg.chain.getByRound(round - 1)
	// last block is not genesis, verify the seed r-1.
	if round != 1 {
		var err error
		lastParentBlock := alg.chain.get(lastBlock.ParentHash, lastBlock.Round-1)
		if lastBlock.Proof != nil {
			// vrf-based seed
			pubkey := recoverPubkey(lastBlock.Signature)
			m := bytes.Join([][]byte{lastParentBlock.Seed, common.Uint2Bytes(lastBlock.Round)}, nil)

			err = pubkey.VerifyVRF(lastBlock.Seed, lastBlock.Proof, m)
		} else if bytes.Compare(lastBlock.Seed, common.Sha256(
			bytes.Join([][]byte{
				lastParentBlock.Seed,
				common.Uint2Bytes(lastBlock.Round)},
				nil)).Bytes()) != 0 {
			// hash-based seed
			err = errors.New("hash seed invalid")
		}
		if err != nil {
			// seed r-1 invalid
			return common.Sha256(bytes.Join([][]byte{lastBlock.Seed, common.Uint2Bytes(lastBlock.Round + 1)}, nil)).Bytes(), nil
		}
	}

	seed, proof = alg.privkey.Evaluate(bytes.Join([][]byte{lastBlock.Seed, common.Uint2Bytes(lastBlock.Round + 1)}, nil))
	return
}

// sortitionSeed returns the selection seed with a refresh interval R.
func (alg *Algorand) sortitionSeed(round uint64) []byte {
	realR := round - 1
	mod := round % R
	if realR < mod {
		realR = 0
	} else {
		realR -= mod
	}

	return alg.chain.getByRound(realR).Seed
}

func (alg *Algorand) publicKey() *PublicKey {
	if pub := alg.pubkey.Load(); pub != nil {
		return pub.(*PublicKey)
	}
	pubkey := alg.privkey.PublicKey()
	alg.pubkey.Store(pubkey)
	return pubkey
}

func (alg *Algorand) Address() common.Address {
	return common.BytesToAddress(alg.publicKey().Bytes())
}

// run performs the all procedures of Algorand algorithm in infinite loop.
func (alg *Algorand) run() {
	forkInterval := time.NewTicker(forkResolveInterval)

	// propose block
	for {
		select {
		case <-alg.quitCh:
			return
		case <-forkInterval.C:
			// periodically resolve fork
		default:
		}
	}

}

// processMain performs the main processing of algorand algorithm.
func (alg *Algorand) processMain() {
	// 1. block proposal
	block := alg.blockProposal(false)

	// 2. init BA with block with the highest priority.
	consensusType, block := alg.BA(alg.round()+1, block)

	// 3. reach consensus on a FINAL or TENTATIVE new block.
	fmt.Printf("reach consensus %d at round %d, block hash %s", consensusType, currRound, block.Hash())

	// 4. append to the chain.
	alg.chain.add(block)

	// TODO: 5. clear cache
}

// processForkResolve performs a special algorand processing to resolve fork.
func (alg *Algorand) processForkResolve() {
	longest := alg.blockProposal(true)
	_, fork := alg.BA(longest.Round, longest)
	alg.chain.resolveFork(fork)
}

// proposeBlock proposes a new block.
func (alg *Algorand) proposeBlock() *Block {
	currRound := alg.round() + 1

	seed, proof := alg.vrfSeed(currRound)
	blk := &Block{
		Round:      currRound,
		Seed:       seed,
		ParentHash: alg.lastBlock().Hash(),
		Author:     alg.publicKey().Address(),
		Time:       time.Now().Unix(),
		Proof:      proof,
	}
	bhash := blk.Hash()
	sign, _ := alg.privkey.Sign(bhash.Bytes())
	blk.Signature = sign
	return blk
}

func (alg *Algorand) proposeFork() *Block {
	longest := alg.lastBlock()
	empty := &Block{
		Round:      alg.round() + 1,
		ParentHash: longest.Hash(),
	}
	return empty
}

// blockProposal performs the block proposal procedure.
func (alg *Algorand) blockProposal(resolveFork bool) *Block {
	var iden string
	if !resolveFork {
		iden = proposer
	} else {
		iden = forkResolve
	}
	round := alg.round() + 1
	vrf, proof, subusers := alg.sortition(alg.sortitionSeed(round), role(iden, round, PROPOSE), expectedBlockProposers, alg.tokenOwn())
	// have been selected.
	if subusers > 0 {
		var (
			newBlk       *Block
			proposalType int
		)
		if !resolveFork {
			newBlk = alg.proposeBlock()
			proposalType = BLOCK_PROPOSAL
		} else {
			newBlk = alg.proposeFork()
			proposalType = FORK_PROPOSAL
		}
		proposal := &Proposal{
			Round:  newBlk.Round,
			Hash:   newBlk.Hash(),
			Prior:  maxPriority(vrf, subusers),
			VRF:    vrf,
			Proof:  proof,
			Pubkey: alg.publicKey().Bytes(),
		}
		alg.peer.setMaxProposal(round, proposal)
		blkMsg, _ := newBlk.Serialize()
		proposalMsg, _ := proposal.Serialize()
		go alg.peer.gossip(BLOCK, blkMsg)
		go alg.peer.gossip(proposalType, proposalMsg)
	}

	// wait for λstepvar + λpriority time to identify the highest priority.
	timeoutForPriority := time.NewTimer(lamdaStepvar + lamdaPriority)
	<-timeoutForPriority.C

	// timeout for block gossiping.
	timeoutForBlockFlying := time.NewTimer(lamdaBlock)
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-timeoutForBlockFlying.C:
			return &Block{
				Round:      round,
				ParentHash: alg.lastBlock().Hash(),
			}
		case <-ticker.C:
			// get the block with the highest priority
			bhash := alg.peer.getMaxProposal(round).Hash
			blk := alg.peer.getBlock(bhash)
			if blk != nil {
				return blk
			}
		}
	}
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

	return subUsers(expectedNum, alg.tokenOwn(), vrf)
}

// committeeVote votes for `value`.
func (alg *Algorand) committeeVote(round uint64, step int, expectedNum int, hash common.Hash) error {
	role := role(committee, round, step)
	vrf, proof, j := alg.sortition(alg.sortitionSeed(round), role, expectedNum, alg.tokenOwn())

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
func (alg *Algorand) BA(round uint64, block *Block) (int8, *Block) {
	var newBlk *Block
	hash := alg.reduction(round, block.Hash())
	hash = alg.binaryBA(round, hash)
	r, _ := alg.countVotes(round, FINAL, finalThreshold, expectedFinalCommitteeMembers, lamdaStep)
	if prevHash := alg.lastBlock().Hash(); hash == emptyHash(round, prevHash) {
		// empty block
		newBlk = &Block{
			Round:      round,
			ParentHash: prevHash,
		}
	} else {
		newBlk = alg.peer.getBlock(hash)
	}
	if r == hash {
		newBlk.Type = FINAL_CONSENSUS
		return FINAL_CONSENSUS, newBlk
	} else {
		newBlk.Type = TENTATIVE_CONSENSUS
		return TENTATIVE_CONSENSUS, newBlk
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
	it := alg.peer.voteIterator(round, step)
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

	votes = alg.verifySort(message.VRF, message.Proof, alg.sortitionSeed(message.Round), role(committee, message.Round, message.Step), expectedNum)
	hash = message.Hash
	vrf = message.VRF
	return
}

// commonCoin computes a coin common to all users.
// It is a procedure to help Algorand recover if an adversary sends faulty messages to the network and prevents the network from coming to consensus.
func (alg *Algorand) commonCoin(round uint64, step int, expectedNum int) int64 {
	minhash := new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength), big.NewInt(0))
	msgList := alg.peer.getIncomingMsgs(round, step)
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
func role(iden string, round uint64, step int) []byte {
	return bytes.Join([][]byte{
		[]byte(iden),
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

func maxPriority(vrf []byte, users int) uint32 {
	var maxPrior uint32
	for i := 1; i <= users; i++ {
		prior := priority(vrf, i)
		if prior > maxPrior {
			maxPrior = prior
		}
	}
	return maxPrior
}

// subUsers return the selected amount of sub-users determined from the mathematics protocol.
func subUsers(expectedNum int, weight uint64, vrf []byte) int {
	binomial := &distuv.Binomial{
		N: float64(weight),
		P: float64(expectedNum) / float64(totalTokenAmount),
	}
	j := 0
	// hash / 2^hashlen ∉ [ ∑0,j B(k;w,p), ∑0,j+1 B(k;w,p))
	hashBig := new(big.Float).SetInt(new(big.Int).SetBytes(vrf))
	maxHash := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength), big.NewInt(0)))
	for {
		lower := new(big.Float).Mul(big.NewFloat(binomial.CDF(float64(j))), maxHash)
		upper := new(big.Float).Mul(big.NewFloat(binomial.CDF(float64(j+1))), maxHash)
		if hashBig.Cmp(lower) >= 0 && hashBig.Cmp(upper) < 0 {
			break
		}
	}
	return j
}

// constructSeed construct a new bytes for vrf generation.
func constructSeed(seed, role []byte) []byte {
	return bytes.Join([][]byte{seed, role}, nil)
}

func emptyHash(round uint64, prev common.Hash) common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		common.Uint2Bytes(round),
		prev.Bytes(),
	}, nil))
}
