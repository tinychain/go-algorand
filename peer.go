package main

import (
	"bytes"
	"github.com/tinychain/algorand/common"
	"sync"
)

var peerPool *PeerPool

type Peer struct {
	algorand      *Algorand
	incomingVotes map[string]*List
	blocks        map[common.Hash]*Block
	maxProposals  map[uint64]*Proposal
}

func newPeer(alg *Algorand) *Peer {
	return &Peer{
		algorand:      alg,
		incomingVotes: make(map[string]*List),
		blocks:        make(map[common.Hash]*Block),
	}
}

func (p *Peer) ID() PID {
	return p.algorand.id
}

func (p *Peer) gossip(typ int, data []byte) {
	ppool := GetPeerPool()

	// TODO: Gossip
	for _, peer := range ppool.peers {
		if peer.ID() == p.ID() {
			continue
		}
		p.handle(typ, data)
	}
}

func (p *Peer) handle(typ int, data []byte) error {
	if typ == BLOCK {
		blk := &Block{}
		blk.Deserialize(data)
		p.blocks[blk.Hash()] = blk
	} else if typ == BLOCK_PROPOSAL {
		bp := &Proposal{}
		bp.Deserialize(data)
		maxProposal := p.maxProposals[p.algorand.round()]
		if bp.Prior <= maxProposal.Prior {
			return nil
		}
		if err := bp.Verify(p.algorand.weight, constructSeed(p.algorand.seed(), role(proposer, p.algorand.round(), PROPOSE))); err != nil {
			return err
		}
		p.maxProposals[p.algorand.round()] = maxProposal
	} else if typ == VOTE {
		vote := &VoteMessage{}
		vote.Deserialize(data)
		list, ok := p.incomingVotes[constructVoteKey(vote.Round, vote.Step)]
		if !ok {
			list = newList()
		}
		list.add(vote)
	}
	return nil
}

// iterator returns the iterator of incoming messages queue.
func (p *Peer) voteIterator(key string) *Iterator {
	return &Iterator{
		list: p.incomingVotes[key],
	}
}

func (p *Peer) getIncomingMsgs(key string) []interface{} {
	l := p.incomingVotes[key]
	if l == nil {
		return nil
	}
	return l.list
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
