package main

import (
	"github.com/tinychain/algorand/common"
	"github.com/urfave/cli"
	"os"
	"time"
)

func main() {
	app := initApp()
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func initApp() *cli.App {
	app := cli.NewApp()
	app.Name = "Algorand demo"
	app.Version = "0.1"
	app.Author = "LowesYang"
	app.Usage = "Algorand simulation demo for different scenario."

	app.Commands = []cli.Command{
		{
			Name:    "regular",
			Aliases: []string{"r"},
			Usage:   "run regular Algorand algorithm",
			Action:  regularRun,
			Flags: []cli.Flag{
				cli.Uint64Flag{
					Name:  "num,n",
					Value: 100,
					Usage: "amount of users",
				},
				cli.Uint64Flag{
					Name:  "token,t",
					Value: 1000,
					Usage: "token balance per users",
				},
				cli.Uint64Flag{
					Name:  "malicious,m",
					Value: 0,
					Usage: "amount of malicious users. Malicious user will use default strategy.",
				},
				cli.IntFlag{
					Name:  "mtype,i",
					Value: 0,
					Usage: "malicious type: 0 Honest, 1 block proposal misbehaving; 2 vote empty block in BA*; 3 vote nothing",
				},
				cli.IntFlag{
					Name:  "latency,l",
					Value: 0,
					Usage: "max network latency(milliseconds). Each user will simulate a random latency between 0 and ${value}",
				},
			},
		},
	}

	return app
}

func regularRun(c *cli.Context) {
	UserAmount = c.Uint64("num")
	TokenPerUser = c.Uint64("token")
	Malicious = c.Uint64("malicious")
	NetworkLatency = c.Int("latency")
	maliciousType := c.Int("type")

	var (
		nodes []*Algorand
		i     = 0
	)
	for ; uint64(i) <= UserAmount-Malicious; i++ {
		node := NewAlgorand(PID(i), Honest)
		go node.Start()
		nodes = append(nodes, node)
	}

	for ; uint64(i) <= UserAmount; i++ {
		node := NewAlgorand(PID(i), maliciousType)
		go node.Start()
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

	snapshot := proposerSelectedHistogram.Snapshot()
	log.Infof("average selected user number in block proposal is %v", snapshot.Mean())
}
