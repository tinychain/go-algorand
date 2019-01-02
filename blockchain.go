package main

import (
	"bytes"
	"github.com/tinychain/algorand/common"
	"sync"
	"time"
)

type Block struct {
	Round      uint64         `json:"round"`
	Height     uint64         `json:"height"`
	ParentHash common.Hash    `json:"parent_hash"`
	Author     common.Address `json:"author"`
	Time       int64          `json:"time"`
}

func (blk *Block) Hash() common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		common.Uint2Bytes(blk.Height),
	}, nil))
}

type Blockchain struct {
	mu           sync.RWMutex
	last         *Block
	genesis      *Block
	heightToHash map[uint64]common.Hash
	blocks       map[common.Hash]*Block
}

func newBlockchain() *Blockchain {
	bc := &Blockchain{
		heightToHash: make(map[uint64]common.Hash),
		blocks:       make(map[common.Hash]*Block),
	}
	bc.init()
	return bc
}

func (bc *Blockchain) init() {
	emptyHash := common.Sha256([]byte{})
	// create genesis
	bc.genesis = &Block{
		Round:      0,
		Height:     0,
		ParentHash: emptyHash,
		Author:     common.HashToAddr(emptyHash),
		Time:       time.Now().Unix(),
	}
}

func (bc *Blockchain) get(hash common.Hash, height uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.blocks[constructBlockKey(hash, height)]
}

func (bc *Blockchain) getByHeight(height uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	hash := bc.heightToHash[height]
	return bc.blocks[constructBlockKey(hash, height)]
}

func (bc *Blockchain) add(blk *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	key := constructBlockKey(blk.Hash(), blk.Height)
	if _, ok := bc.blocks[key]; ok {
		return
	}
	bc.blocks[key] = blk
	bc.heightToHash[blk.Height] = blk.Hash()
}

func constructBlockKey(hash common.Hash, height uint64) common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		hash.Bytes(),
		common.Uint2Bytes(height),
	}, nil))
}
