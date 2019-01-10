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
	app.Author = "yyh1102"
	app.Usage = "Algorand simulation demo for different scenario."

	app.Commands = []cli.Command{
		{
			Name:    "regular",
			Aliases: []string{"r"},
			Usage:   "run regular process of Algorand algorithm",
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
					Usage: "amount of malicious users",
				},
				cli.IntFlag{
					Name:  "latency,l",
					Value: 0,
					Usage: "network latency",
				},
			},
		},
	}

	return app
}

func regularRun(c *cli.Context) {
	userAmount := c.Uint64("num")
	tokenPerUser := c.Uint64("token")
	UserAmount = userAmount
	TokenPerUser = tokenPerUser

	var nodes []*Algorand
	for i := 1; uint64(i) <= userAmount; i++ {
		node := NewAlgorand(PID(i))
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
}
