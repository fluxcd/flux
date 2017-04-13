package git

import (
	"errors"
	"fmt"
	"strings"

	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
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

// Convenience function for doing this, since we use it in a couple of
// places
func Fingerprint(keyData []byte, algo string) (string, error) {
	key, err := ssh.ParseRawPrivateKey(keyData)
	if err != nil {
		return "", err
	}
	privKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("unable to extract key from bytes")
	}
	pubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", err
	}

	switch algo {
	case "md5":
		hash := md5.Sum(pubKey.Marshal())
		fingerprint := ""
		for i, b := range hash {
			fingerprint = fmt.Sprintf("%s%0.2x", fingerprint, b)
			if i < len(hash)-1 {
				fingerprint = fingerprint + ":"
			}
		}
		return fingerprint, nil
	case "sha256":
		hash := sha256.Sum256(pubKey.Marshal())
		return strings.TrimRight(base64.StdEncoding.EncodeToString(hash[:]), "="), nil
	default:
		return "", errors.New("unknown fingerprint hash algo (should be md5|sha256): " + algo)
	}
}
