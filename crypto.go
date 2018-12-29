package main

import (
	"crypto"
	"crypto/rand"
	"fmt"
	"github.com/fatechain/fatechain/common"
	"golang.org/x/crypto/ed25519"
	rander "math/rand"
	"time"
)

type PublicKey struct {
	pk ed25519.PublicKey
}

func (pub *PublicKey) Bytes() []byte {
	return pub.pk
}

func (pub *PublicKey) VerifySign(m, sign []byte) error {
	signature := sign[ed25519.PublicKeySize:]
	if ok := ed25519.Verify(pub.pk, m, signature); !ok {
		return fmt.Errorf("signature invalid")
	}
	return nil
}

func (pub *PublicKey) VerifyVRF(vrf, proof, m []byte) error {
	return nil
}

type PrivateKey struct {
	sk ed25519.PrivateKey
}

func (priv *PrivateKey) PublicKey() *PublicKey {
	return &PublicKey{priv.sk.Public().(ed25519.PublicKey)}
}

func (priv *PrivateKey) Sign(m []byte) ([]byte, error) {
	sign, err := priv.sk.Sign(rand.Reader, m, crypto.Hash(0))
	if err != nil {
		return nil, err
	}
	pubkey := priv.sk.Public().(ed25519.PublicKey)
	return append(pubkey, sign...), nil
}

func (priv *PrivateKey) Evaluate(m []byte) (value, proof []byte) {
	rander.Seed(time.Now().UnixNano())
	value, proof = common.UintToBytes(rander.Uint64()), common.UintToBytes(rander.Uint64())
	return
}

func newKeyPair() (*PublicKey, *PrivateKey, error) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	return &PublicKey{pk}, &PrivateKey{sk}, nil
}
