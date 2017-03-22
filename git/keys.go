package git

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
)

// KeySize is the size of generated private keys.
var KeySize = 4096

type KeyGenerator interface {
	Generate() (privateKey []byte, err error)
}

func NewKeyGenerator() KeyGenerator {
	return &key{}
}

type key struct{}

// Private Key generated is PEM encoded
// Public key is generated as part of the get-config methods
func (k *key) Generate() (privateKeyB []byte, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return
	}

	// generate and write private key as PEM
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	privateKeyB = pem.EncodeToMemory(privateKeyPEM)
	return
}
