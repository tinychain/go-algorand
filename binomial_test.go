package main

import (
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
	"gonum.org/v1/gonum/stat/distuv"
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
	b.ResetTimer()
	binomial_1 := NewBinomial(10000, 1, 100)
	for i := 0; i < b.N; i++ {
		binomial_1.CDF(int64(i))
	}
}

func BenchmarkBinomial_distuv(b *testing.B) {
	b.ResetTimer()
	binomial_2 := &distuv.Binomial{
		N: 10000,
		P: float64(1) / float64(100),
	}
	for i := 0; i < b.N; i++ {
		binomial_2.CDF(float64(i))
	}
}
