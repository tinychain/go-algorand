package main

import (
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestBinomial_CDF(t *testing.T) {
	binomial := NewBinomial(5, 1, 2)
	b0 := big.NewRat(1, 32)
	b1 := big.NewRat(6, 32)
	b2 := big.NewRat(16, 32)
	b3 := big.NewRat(26, 32)
	b4 := big.NewRat(31, 32)
	b5 := big.NewRat(32, 32)
	assert.Equal(t, 0, binomial.CDF(0).Cmp(b0))
	assert.Equal(t, 0, binomial.CDF(1).Cmp(b1))
	assert.Equal(t, 0, binomial.CDF(2).Cmp(b2))
	assert.Equal(t, 0, binomial.CDF(3).Cmp(b3))
	assert.Equal(t, 0, binomial.CDF(4).Cmp(b4))
	assert.Equal(t, 0, binomial.CDF(5).Cmp(b5))
}

func BenchmarkBinomial_CDF(b *testing.B) {

}
