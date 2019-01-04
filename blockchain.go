package main

import (
	"bytes"
	"encoding/json"
	"github.com/tinychain/algorand/common"
	"sync"
	"time"
)

type Block struct {
	Round      uint64         `json:"round"` // block round, namely height
	ParentHash common.Hash    `json:"parent_hash"`
	Author     common.Address `json:"author"`
	Time       int64          `json:"time"`  // block timestamp
	Seed       []byte         `json:"seed"`  // vrf-based seed for next round
	Proof      []byte         `json:"proof"` // proof of vrf-based seed
	Type       int8           `json:"type"`  // `FINAL` or `TENTATIVE`
	Signature  []byte         `json:"signature"`
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
	mu          sync.RWMutex
	last        *Block
	genesis     *Block
	roundToHash map[uint64]common.Hash
	blocks      map[common.Hash]*Block
}

func newBlockchain() *Blockchain {
	bc := &Blockchain{
		roundToHash: make(map[uint64]common.Hash),
		blocks:      make(map[common.Hash]*Block),
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
	return bc.blocks[constructBlockKey(hash, round)]
}

func (bc *Blockchain) getByRound(round uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	hash := bc.roundToHash[round]
	return bc.blocks[constructBlockKey(hash, round)]
}

func (bc *Blockchain) add(blk *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	key := constructBlockKey(blk.Hash(), blk.Round)
	if _, ok := bc.blocks[key]; ok {
		return
	}
	bc.blocks[key] = blk
	bc.roundToHash[blk.Round] = blk.Hash()
}

func constructBlockKey(hash common.Hash, round uint64) common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		hash.Bytes(),
		common.Uint2Bytes(round),
	}, nil))
}
