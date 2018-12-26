package algorand

import (
	"bytes"
	"github.com/tinychain/algorand/common"
	"sync"
	"time"
)

type Block struct {
	Height     uint64         `json:"height"`
	ParentHash common.Hash    `json:"parent_hash"`
	Author     common.Address `json:"author"`
	Time       time.Duration  `json:"time"`
}

func (blk *Block) Hash() common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		common.Uint2Bytes(blk.Height),
	}, nil))
}

type Blockchain struct {
	mu           sync.RWMutex
	last         *Block
	heightToHash map[uint64]common.Hash
	blocks       map[common.Hash]*Block
}

func (bc *Blockchain) get(hash common.Hash, height uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.blocks[constructKey(hash, height)]
}

func (bc *Blockchain) getByHeight(height uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	hash := bc.heightToHash[height]
	return bc.blocks[constructKey(hash, height)]
}

func (bc *Blockchain) append(blk *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	key := constructKey(blk.Hash(), blk.Height)
	bc.blocks[key] = blk
	bc.heightToHash[blk.Height] = blk.Hash()
}

func constructKey(hash common.Hash, height uint64) common.Hash {
	return common.Sha256(bytes.Join([][]byte{
		hash.Bytes(),
		common.Uint2Bytes(height),
	}, nil))
}
