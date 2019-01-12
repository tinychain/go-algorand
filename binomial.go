package main

import (
	"math/big"
)

type Binomial struct {
	N *big.Int
	P *big.Rat

	probCache map[int64]*big.Rat
}

func NewBinomial(n, pn, pd int64) *Binomial {
	return &Binomial{
		N:         big.NewInt(n),
		P:         big.NewRat(pn, pd),
		probCache: make(map[int64]*big.Rat),
	}
}

func (b *Binomial) CDF(j int64) *big.Rat {
	if j > b.N.Int64() {
		return new(big.Rat).SetInt64(1)
	}

	k := int64(0)
	res := new(big.Rat).SetInt64(0)
	for k <= j {
		prob := b.prob(k)
		res = res.Add(res, prob)
		k++
	}
	return res
}

func (b *Binomial) prob(k int64) *big.Rat {
	if prob, exist := b.probCache[k]; exist {
		return prob
	}
	numer := b.P.Num()   // pn
	denom := b.P.Denom() // pd

	bk := big.NewInt(k)              // k
	sbk := new(big.Int).Sub(b.N, bk) // n-k

	// calculate p^k * p^(n-k)
	lnum := new(big.Int).Exp(numer, bk, nil)
	rnum := new(big.Int).Exp(new(big.Int).Sub(denom, numer), sbk, nil)
	resNum := new(big.Int).Mul(lnum, rnum)
	//fmt.Printf("resNum %v\n", resNum.Int64())
	resDenom := new(big.Int).Exp(denom, b.N, nil)
	//fmt.Printf("resDenom %v\n", resDenom.Int64())

	mulRat := new(big.Rat).SetFrac(resNum, resDenom)

	// calculate C(n,k)
	bino := new(big.Int).Binomial(b.N.Int64(), k)

	// prob = C(n,k) * p^k * p^(n-k)
	prob := new(big.Rat).Mul(new(big.Rat).SetInt(bino), mulRat)

	b.probCache[k] = prob
	return prob
}
