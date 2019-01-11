package main

import (
	"bytes"
	"github.com/tinychain/algorand/common"
	"math/rand"
	"sync"
	"time"
)

var peerPool *PeerPool

type Peer struct {
	algorand *Algorand

	vmu sync.RWMutex // vote msg mutex
	bmu sync.RWMutex // block list mutex
	pmu sync.RWMutex // proposal list mutex

	incomingVotes map[string]*List
	blocks        map[common.Hash]*Block
	maxProposals  map[uint64]*Proposal
}

func newPeer(alg *Algorand) *Peer {
	return &Peer{
		algorand:      alg,
		incomingVotes: make(map[string]*List),
		blocks:        make(map[common.Hash]*Block),
		maxProposals:  make(map[uint64]*Proposal),
	}
}

func (p *Peer) start() {
	GetPeerPool().add(p)
}

func (p *Peer) stop() {
	GetPeerPool().remove(p)
}

func (p *Peer) ID() PID {
	return p.algorand.id
}

func (p *Peer) gossip(typ int, data []byte) {
	peers := GetPeerPool().getPeers()
	// simulate gossiping
	for _, peer := range peers {
		if peer.ID() == p.ID() {
			continue
		}
		if NetworkLatency != 0 {
			time.Sleep(time.Duration(rand.Intn(NetworkLatency)) * time.Millisecond)
		}
		go peer.handle(typ, data)
	}
}

// halfGossip gossips the message to a half of remote peers it observes.
// This gossip will be used by block proposal malicious users.
func (p *Peer) halfGossip(typ int, data []byte, half int) {
	if half > 1 {
		return
	}
	peers := GetPeerPool().getPeers()
	var begin, end int
	begin = len(peers) / 2 * half
	if half == 0 {
		end = len(peers) / 2
	} else {
		end = len(peers)
	}
	for ; begin < end; begin++ {
		peer := peers[begin]
		if peer.ID() == p.ID() {
			continue
		}
		if NetworkLatency != 0 {
			time.Sleep(time.Duration(rand.Intn(NetworkLatency)) * time.Millisecond)
		}
		go peer.handle(typ, data)
	}
}

func (p *Peer) handle(typ int, data []byte) error {
	if typ == BLOCK {
		blk := &Block{}
		blk.Deserialize(data)
		p.addBlock(blk.Hash(), blk)
	} else if typ == BLOCK_PROPOSAL || typ == FORK_PROPOSAL {
		bp := &Proposal{}
		bp.Deserialize(data)
		p.pmu.RLock()
		maxProposal := p.maxProposals[bp.Round]
		p.pmu.RUnlock()
		if maxProposal != nil {
			if (typ == BLOCK_PROPOSAL && bytes.Compare(bp.Prior, maxProposal.Prior) <= 0) ||
				(typ == FORK_PROPOSAL && bp.Round <= maxProposal.Round) {
				return nil
			}
		}
		if err := bp.Verify(p.algorand.weight(bp.Address()), constructSeed(p.algorand.sortitionSeed(bp.Round), role(proposer, bp.Round, PROPOSE)));
			err != nil {
			log.Errorf("block proposal verification failed, %s", err)
			return err
		}
		p.setMaxProposal(bp.Round, bp)
	} else if typ == VOTE {
		vote := &VoteMessage{}
		vote.Deserialize(data)
		key := constructVoteKey(vote.Round, vote.Step)
		p.vmu.RLock()
		list, ok := p.incomingVotes[key]
		p.vmu.RUnlock()
		if !ok {
			list = newList()
		}
		list.add(vote)
		p.vmu.Lock()
		p.incomingVotes[key] = list
		p.vmu.Unlock()
	}
	return nil
}

// iterator returns the iterator of incoming messages queue.
func (p *Peer) voteIterator(round uint64, step int) *Iterator {
	key := constructVoteKey(round, step)
	p.vmu.RLock()
	list, ok := p.incomingVotes[key]
	p.vmu.RUnlock()
	if !ok {
		list = newList()
		p.vmu.Lock()
		p.incomingVotes[key] = list
		p.vmu.Unlock()
	}
	return &Iterator{
		list: list,
	}
}

func (p *Peer) getIncomingMsgs(round uint64, step int) []interface{} {
	p.vmu.RLock()
	defer p.vmu.RUnlock()
	l := p.incomingVotes[constructVoteKey(round, step)]
	if l == nil {
		return nil
	}
	return l.list
}

func (p *Peer) getBlock(hash common.Hash) *Block {
	p.bmu.RLock()
	defer p.bmu.RUnlock()
	return p.blocks[hash]
}

func (p *Peer) addBlock(hash common.Hash, blk *Block) {
	p.bmu.Lock()
	defer p.bmu.Unlock()
	p.blocks[hash] = blk
}

func (p *Peer) setMaxProposal(round uint64, proposal *Proposal) {
	p.pmu.Lock()
	defer p.pmu.Unlock()
	//log.Infof("node %d set max proposal #%d %s", p.ID(), proposal.Round, proposal.Hash)
	p.maxProposals[round] = proposal
}

func (p *Peer) getMaxProposal(round uint64) *Proposal {
	p.pmu.RLock()
	defer p.pmu.RUnlock()
	return p.maxProposals[round]
}

func (p *Peer) clearProposal(round uint64) {
	p.pmu.Lock()
	defer p.pmu.Unlock()
	delete(p.maxProposals, round)
}

func constructVoteKey(round uint64, step int) string {
	return string(bytes.Join([][]byte{
		common.Uint2Bytes(round),
		common.Uint2Bytes(uint64(step)),
	}, nil))
}

type List struct {
	mu   sync.RWMutex
	list []interface{}
}

func newList() *List {
	return &List{}
}

func (l *List) add(el interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, el)
}

func (l *List) get(index int) interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index >= len(l.list) {
		return nil
	}
	return l.list[index]
}

type Iterator struct {
	list  *List
	index int
}

func (it *Iterator) next() interface{} {
	el := it.list.get(it.index)
	if el == nil {
		return nil
	}
	it.index++
	return el
}

type PeerPool struct {
	mu    sync.Mutex
	peers map[PID]*Peer
}

func GetPeerPool() *PeerPool {
	if peerPool == nil {
		peerPool = &PeerPool{
			peers: make(map[PID]*Peer),
		}
	}
	return peerPool
}

func (pool *PeerPool) add(peer *Peer) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	pool.peers[peer.ID()] = peer
}

func (pool *PeerPool) remove(peer *Peer) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	delete(pool.peers, peer.ID())
}

func (pool *PeerPool) getPeers() []*Peer {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	var peers []*Peer
	for _, peer := range pool.peers {
		peers = append(peers, peer)
	}
	return peers
}
