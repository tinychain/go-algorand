package main

import (
	"bytes"
	"github.com/tinychain/algorand/common"
	"sync"
)

type Peer struct {
	incomingMsgs map[string]*List
}

func newPeer() *Peer {
	return &Peer{
		incomingMsgs: make(map[string]*List),
	}
}

func (p *Peer) gossip(typ int, data []byte) {
	basic := &BasicMessage{
		Type: typ,
		Data: data,
	}

	// TODO: Gossip
}

// iterator returns the iterator of incoming messages queue.
func (p *Peer) iterator(key string) *Iterator {
	return &Iterator{
		list: p.incomingMsgs[key],
	}
}

func (p *Peer) getIncomingMsgs(key string) []interface{} {
	l := p.incomingMsgs[key]
	if l == nil {
		return nil
	}
	return l.list
}

func constructMsgKey(round uint64, step int) string {
	return string(bytes.Join([][]byte{
		common.Uint2Bytes(round),
		common.Uint2Bytes(uint64(step)),
	}, nil))
}

type List struct {
	mu   sync.RWMutex
	list []interface{}
}

func newQueue() *List {
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
