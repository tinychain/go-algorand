package main

import (
	"github.com/tinychain/algorand/common"
	"time"
)

func main() {
	var nodes []*Algorand
	for i := 1; i <= userAmount; i++ {
		node := NewAlgorand(PID(i))
		go node.start()
		nodes = append(nodes, node)
	}

	ticker := time.NewTicker(20 * time.Second)
	for {
		select {
		case <-ticker.C:
			printChainInfo(nodes)
		}
	}
}

type meta struct {
	round uint64
	hash  common.Hash
}

func printChainInfo(nodes []*Algorand) {
	chains := make(map[meta][]PID)
	for _, node := range nodes {
		last := node.chain.last
		key := meta{last.Round, last.Hash()}
		chains[key] = append(chains[key], node.id)
	}

	for meta, pids := range chains {
		log.Infof("%d nodes reach consensus on chain round %d, hash %s", len(pids), meta.round, meta.hash)
	}
}
