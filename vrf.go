package main

type PublicKey interface {
	Bytes() []byte
	VerifySign(m, sign []byte) error
}

type PrivateKey interface {
	Bytes() []byte
	PublicKey() PublicKey
	Sign(m []byte) ([]byte, error)
}

// RecoverPubkey recovers public key from a given signature.
func RecoverPubkey(sign []byte) (PublicKey, error) {

}
