package main

import (
	"testing"
	"github.com/tinychain/algorand/common"
	"math/rand"
	"time"
	"fmt"
)

func BenchmarkSubUsers(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < b.N; i++ {
		hash := common.BytesToHash(common.Uint2Bytes(rand.Uint64()))
		subUsers(26, 1000, hash.Bytes())
	}
}

func TestSubUsersSingle(t *testing.T) {
	begin := time.Now().UnixNano()
	subUsers(26, 1000, common.BytesToHash(common.Uint2Bytes(uint64(0))).Bytes())
	fmt.Printf("subusers cost %v\n", time.Now().UnixNano()-begin)
}
