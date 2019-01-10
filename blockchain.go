package main

import (
	"bytes"
	"encoding/json"
	"github.com/tinychain/algorand/common"
	"math/rand"
	"sync"
	"time"
)

type Block struct {
	Round       uint64         `json:"round"`        // block round, namely height
	ParentHash  common.Hash    `json:"parent_hash"`  // parent block hash
	Author      common.Address `json:"author"`       // proposer address
	AuthorVRF   []byte         `json:"author_vrf"`   // sortition hash
	AuthorProof []byte         `json:"author_proof"` // sortition hash proof
	Time        int64          `json:"time"`         // block timestamp
	Seed        []byte         `json:"seed"`         // vrf-based seed for next round
	Proof       []byte         `json:"proof"`        // proof of vrf-based seed

	// don't induce in hash
	Type      int8   `json:"type"`      // `FINAL` or `TENTATIVE`
	Signature []byte `json:"signature"` // signature of block
}

func (blk *Block) Hash() common.Hash {
	data := bytes.Join([][]byte{
		common.Uint2Bytes(blk.Round),
		blk.ParentHash.Bytes(),
	}, nil)

	if !blk.Author.Nil() {
		data = append(data, bytes.Join([][]byte{
			blk.Author.Bytes(),
			blk.AuthorVRF,
			blk.AuthorProof,
		}, nil)...)
	}

	if blk.Time != 0 {
		data = append(data, common.Uint2Bytes(uint64(blk.Time))...)
	}

	if blk.Seed != nil {
		data = append(data, blk.Seed...)
	}

	if blk.Proof != nil {
		data = append(data, blk.Proof...)
	}

	return common.Sha256(data)
}

func (blk *Block) RecoverPubkey() *PublicKey {
	return recoverPubkey(blk.Signature)
}

func (blk *Block) Serialize() ([]byte, error) {
	return json.Marshal(blk)
}

func (blk *Block) Deserialize(data []byte) error {
	return json.Unmarshal(data, blk)
}

type Blockchain struct {
	mu      sync.RWMutex
	last    *Block
	genesis *Block
	blocks  map[uint64]map[common.Hash]*Block
}

func newBlockchain() *Blockchain {
	bc := &Blockchain{
		blocks: make(map[uint64]map[common.Hash]*Block),
	}
	bc.init()
	return bc
}

func (bc *Blockchain) init() {
	rand.Seed(time.Now().Unix())
	emptyHash := common.Sha256([]byte{})
	// create genesis
	bc.genesis = &Block{
		Round:      0,
		Seed:       common.Uint2Bytes(rand.Uint64()),
		ParentHash: emptyHash,
		Author:     common.HashToAddr(emptyHash),
		Time:       time.Now().Unix(),
	}
	bc.add(bc.genesis)
}

func (bc *Blockchain) get(hash common.Hash, round uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	blocks := bc.blocks[round]
	if blocks != nil {
		return blocks[hash]
	}
	return nil
}

func (bc *Blockchain) getByRound(round uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	last := bc.last
	for round > 0 {
		if last.Round == round {
			return last
		}
		last = bc.get(last.ParentHash, round-1)
		round--
	}
	return last
}

func (bc *Blockchain) add(blk *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	blocks := bc.blocks[blk.Round]
	if blocks == nil {
		blocks = make(map[common.Hash]*Block)
	}
	blocks[blk.Hash()] = blk
	bc.blocks[blk.Round] = blocks
	if bc.last == nil || blk.Round > bc.last.Round {
		bc.last = blk
	}
}

func (bc *Blockchain) resolveFork(fork *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.last = fork
}
