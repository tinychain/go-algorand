package main

import (
	"time"
)

const (
	userAmount = 10
)

func main() {
	var nodes []*Algorand
	for i := 1; i <= userAmount; i++ {
		node := NewAlgorand(PID(i))
		go node.start()
		nodes = append(nodes, node)
	}

	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			printChainInfo(nodes)
		}
	}
}

func printChainInfo(nodes []*Algorand) {
	chains := make(map[uint64][]PID)
	for _, node := range nodes {
		chains[node.chain.last.Round] = append(chains[node.chain.last.Round], node.id)
	}

	for round, pids := range chains {
		log.Infof("%d nodes reach consensus on chain round %d", len(pids), round)
	}
}
