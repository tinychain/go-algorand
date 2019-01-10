package main

import (
	"bytes"
	"errors"
	"github.com/tinychain/algorand/common"
	"gonum.org/v1/gonum/stat/distuv"
	"math/big"
	"time"
)

var (
	log = common.GetLogger("algorand")

	errCountVotesTimeout = errors.New("count votes timeout")
)

type PID int

type Algorand struct {
	id      PID
	privkey *PrivateKey
	pubkey  *PublicKey

	chain       *Blockchain
	peer        *Peer
	quitCh      chan struct{}
	hangForever chan struct{}
}

func NewAlgorand(id PID) *Algorand {
	pub, priv, _ := NewKeyPair()
	alg := &Algorand{
		id:      id,
		privkey: priv,
		pubkey:  pub,
		chain:   newBlockchain(),
	}
	alg.peer = newPeer(alg)
	return alg
}

func (alg *Algorand) start() {
	alg.quitCh = make(chan struct{})
	alg.hangForever = make(chan struct{})
	alg.peer.start()
	go alg.run()
}

func (alg *Algorand) stop() {
	close(alg.quitCh)
	close(alg.hangForever)
	alg.peer.stop()
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
func (alg *Algorand) vrfSeed(round uint64) (seed, proof []byte, err error) {
	if round == 0 {
		return alg.chain.genesis.Seed, nil, nil
	}
	lastBlock := alg.chain.getByRound(round - 1)
	// last block is not genesis, verify the seed r-1.
	if round != 1 {
		lastParentBlock := alg.chain.get(lastBlock.ParentHash, lastBlock.Round-1)
		if lastBlock.Proof != nil {
			// vrf-based seed
			pubkey := recoverPubkey(lastBlock.Signature)
			m := bytes.Join([][]byte{lastParentBlock.Seed, common.Uint2Bytes(lastBlock.Round)}, nil)
			err = pubkey.VerifyVRF(lastBlock.Proof, m)
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
			return common.Sha256(bytes.Join([][]byte{lastBlock.Seed, common.Uint2Bytes(lastBlock.Round + 1)}, nil)).Bytes(), nil, nil
		}
	}

	seed, proof, err = alg.privkey.Evaluate(bytes.Join([][]byte{lastBlock.Seed, common.Uint2Bytes(lastBlock.Round + 1)}, nil))
	return
}

func (alg *Algorand) emptyBlock(round uint64, prevHash common.Hash) *Block {
	return &Block{
		Round:      round,
		ParentHash: prevHash,
	}
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

func (alg *Algorand) Address() common.Address {
	return common.BytesToAddress(alg.pubkey.Bytes())
}

// run performs the all procedures of Algorand algorithm in infinite loop.
func (alg *Algorand) run() {
	// sleep 1 millisecond for all peers ready.
	time.Sleep(1 * time.Millisecond)

	forkInterval := time.NewTicker(forkResolveInterval)

	// propose block
	for {
		select {
		case <-alg.quitCh:
			return
		case <-forkInterval.C:
			// periodically resolve fork
			alg.processForkResolve()
		default:
			alg.processMain()
		}
	}

}

// processMain performs the main processing of algorand algorithm.
func (alg *Algorand) processMain() {
	currRound := alg.round() + 1
	log.Infof("node %d begin to perform consensus at round %d", alg.id, currRound)
	// 1. block proposal
	block := alg.blockProposal(false)
	log.Debugf("node %d init BA with block #%d %s", alg.id, block.Round, block.Hash())

	// 2. init BA with block with the highest priority.
	consensusType, block := alg.BA(currRound, block)

	// 3. reach consensus on a FINAL or TENTATIVE new block.
	log.Infof("node %d reach consensus %d at round %d, block hash %s", alg.id, consensusType, currRound, block.Hash())

	// 4. append to the chain.
	alg.chain.add(block)

	// TODO: 5. clear cache
}

// processForkResolve performs a special algorand processing to resolve fork.
func (alg *Algorand) processForkResolve() {
	// propose fork
	longest := alg.blockProposal(true)
	// init BA with a highest priority fork
	_, fork := alg.BA(longest.Round, longest)
	// commit fork
	alg.chain.resolveFork(fork)
}

// proposeBlock proposes a new block.
func (alg *Algorand) proposeBlock() *Block {
	currRound := alg.round() + 1

	seed, proof, err := alg.vrfSeed(currRound)
	if err != nil {
		return nil
	}
	blk := &Block{
		Round:      currRound,
		Seed:       seed,
		ParentHash: alg.lastBlock().Hash(),
		Author:     alg.pubkey.Address(),
		Time:       time.Now().Unix(),
		Proof:      proof,
	}
	bhash := blk.Hash()
	sign, _ := alg.privkey.Sign(bhash.Bytes())
	blk.Signature = sign
	log.Infof("node %d propose a new block #%d %s", alg.id, blk.Round, blk.Hash())
	return blk
}

func (alg *Algorand) proposeFork() *Block {
	longest := alg.lastBlock()
	return alg.emptyBlock(alg.round()+1, longest.Hash())
}

// blockProposal performs the block proposal procedure.
func (alg *Algorand) blockProposal(resolveFork bool) *Block {
	round := alg.round() + 1
	vrf, proof, subusers := alg.sortition(alg.sortitionSeed(round), role(proposer, round, PROPOSE), expectedBlockProposers, alg.tokenOwn())
	// have been selected.
	log.Infof("node %d get %d sub-users in block proposal", alg.id, subusers)
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
		if newBlk == nil {
			return alg.emptyBlock(round, alg.lastBlock().Hash())
		}
		proposal := &Proposal{
			Round:  newBlk.Round,
			Hash:   newBlk.Hash(),
			Prior:  maxPriority(vrf, subusers),
			VRF:    vrf,
			Proof:  proof,
			Pubkey: alg.pubkey.Bytes(),
		}
		alg.peer.setMaxProposal(round, proposal)
		alg.peer.addBlock(newBlk.Hash(), newBlk)
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
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-timeoutForBlockFlying.C:
			// empty block
			return alg.emptyBlock(round, alg.lastBlock().Hash())
		case <-ticker.C:
			// get the block with the highest priority
			pp := alg.peer.getMaxProposal(round)
			if pp == nil {
				continue
			}
			blk := alg.peer.getBlock(pp.Hash)
			if blk != nil {
				return blk
			}
		}
	}
}

// sortition runs cryptographic selection procedure and returns vrf,proof and amount of selected sub-users.
func (alg *Algorand) sortition(seed, role []byte, expectedNum int, weight uint64) (vrf, proof []byte, selected int) {
	vrf, proof, _ = alg.privkey.Evaluate(constructSeed(seed, role))
	selected = subUsers(expectedNum, weight, vrf)
	return
}

// verifySort verifies the vrf and returns the amount of selected sub-users.
func (alg *Algorand) verifySort(vrf, proof, seed, role []byte, expectedNum int) int {
	if err := alg.pubkey.VerifyVRF(proof, constructSeed(seed, role)); err != nil {
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
		newBlk = alg.emptyBlock(round, prevHash)
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
	defer func() {
		log.Infof("node %d complete binaryBA with %d steps", alg.id, step)
	}()
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

	log.Infof("reach the maxstep hang forever")
	// hang forever
	<-alg.hangForever
	return common.Hash{}
}

// countVotes counts votes for round and step.
func (alg *Algorand) countVotes(round uint64, step int, threshold float64, expectedNum int, timeout time.Duration) (common.Hash, error) {
	expired := time.NewTimer(timeout)
	counts := make(map[common.Hash]int)
	voters := make(map[string]struct{})
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
			if _, exist := voters[string(pubkey.pk)]; exist || votes == 0 {
				continue
			}
			voters[string(pubkey.pk)] = struct{}{}
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

// maxPriority returns the highest priority of block proposal.
func maxPriority(vrf []byte, users int) []byte {
	var maxPrior []byte
	for i := 1; i <= users; i++ {
		prior := common.Sha256(bytes.Join([][]byte{vrf, common.Uint2Bytes(uint64(i))}, nil)).Bytes()
		if bytes.Compare(prior, maxPrior) > 0 {
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
	maxHash := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(common.HashLength*8), big.NewInt(0)))
	for uint64(j) <= weight {
		lower := new(big.Float).Mul(big.NewFloat(binomial.CDF(float64(j))), maxHash)
		upper := new(big.Float).Mul(big.NewFloat(binomial.CDF(float64(j+1))), maxHash)
		//log.Infof("hashBig is %v, 2^hashlen %v, lower bound %f, upper bound %f", hb, maxH, fl, fu)
		if hashBig.Cmp(lower) >= 0 && hashBig.Cmp(upper) < 0 {
			break
		}
		j++
	}
	if uint64(j) > weight {
		j = 0
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
