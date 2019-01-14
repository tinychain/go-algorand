# Algorand
Go implementation of Algorand algorithm based on the paper [gilad-algorand-eprint](https://people.csail.mit.edu/nickolai/papers/gilad-algorand-eprint.pdf). 

It's just a POC demo and cannot be used in PROD ENV.

## Analysis
- OS: Mac OSX Mojava 
- CPU: Intel Core i5 8th 2.3GHz 4 cores.
- RAM: 16GB

Some problems founded when run on a single machine:
   - Binomial distribution function(implemented with `big.Rat`) used by `sortision` is too slow!!!.
   - Due to the binomial bottleneck, the amount of proposers or committee members are often too small to make the votes cross threshold.

## How to run
Running regular Algorand algorithm
```shell
go build
./algorand regular
```

And you can type
```shell
./algorand regular -h 
```
to check and fill some custom options to meet your demand.

```shell
NAME:
   main regular - run regular Algorand algorithm

USAGE:
   main regular [command options] [arguments...]

OPTIONS:
   --num value, -n value        amount of users (default: 100)
   --token value, -t value      token balance per users (default: 1000)
   --malicious value, -m value  amount of malicious users. Malicious user will use default strategy. (default: 0)
   --mtype value, -i value      malicious type: 0 Honest, 1 block proposal misbehaving; 2 vote empty block in BA*; 3 vote nothing (default: 0)
   --latency value, -l value    max network latency(milliseconds). Each user will simulate a random latency between 0 and ${value} (default: 0)
```

#### Options
- `--num,-n`: The amount of users in Algorand.
- `--token,-t`: The token balance provided for every user.
- `--malicious,-m`: The amount of malicious users.
- `--mtype,-i`: The malicious type of the malicious node
    - `0`: It's an honest user.
    - `1`: When the user happens to be the proposer with the highest priority, it will send one version of block to half of its remote peers, and another version to the others.(NEED TESTING)
    - `2`: During the BA* period, the user will vote empty block forever.(NEED TESTING)
    - `3`: The user attends in proposers and committee selection, but votes nothing.(NEED TESTING)
- `--latency,-l`: The random upper bound of network latency. The peer will simulate a random delay between 0 and ${value} when sending messages.(NEED TESTING)

## Docs
- [Algorand Analysis](docs/剖析Algorand.md)
