package main

import (
	"bytes"
	"encoding/json"
	"github.com/tinychain/algorand/common"
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
	return common.Sha256(bytes.Join([][]byte{
		common.Uint2Bytes(blk.Round),
		blk.ParentHash.Bytes(),
		blk.Author.Bytes(),
		common.Uint2Bytes(uint64(blk.Time)),
		blk.Seed,
		blk.Proof,
	}, nil))
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
	emptyHash := common.Sha256([]byte{})
	// create genesis
	bc.genesis = &Block{
		Round:      0,
		ParentHash: emptyHash,
		Author:     common.HashToAddr(emptyHash),
		Time:       time.Now().Unix(),
	}
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
	for round != 0 {
		if last.Round == round {
			return last
		}
		last = bc.blocks[round-1][last.ParentHash]
		round--
	}
	return nil
}

func (bc *Blockchain) add(blk *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	blocks := bc.blocks[blk.Round]
	if blocks == nil {
		blocks = make(map[common.Hash]*Block)
	}
	blocks[blk.Hash()] = blk
	if blk.Round > bc.last.Round {
		bc.last = blk
	}
}

func (bc *Blockchain) resolveFork(fork *Block) {
	bc.last = fork
}
