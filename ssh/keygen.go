package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"regexp"
	"strconv"

	"github.com/spf13/pflag"
)

// OptionalValue is an extension of pflag.Value that remembers if it has been
// set.
type OptionalValue interface {
	pflag.Value
	Specified() bool
}

// KeyBitsValue is an OptionalValue allowing specification of the -b argument
// to ssh-keygen.
type KeyBitsValue struct {
	specified bool
	keyBits   uint64
}

func (kbv *KeyBitsValue) String() string {
	return strconv.FormatUint(kbv.keyBits, 10)
}

func (kbv *KeyBitsValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return err
	}

	kbv.keyBits = v
	kbv.specified = true

	return nil
}

func (kbv *KeyBitsValue) Type() string {
	return "uint64"
}

func (kbv *KeyBitsValue) Specified() bool {
	return kbv.specified
}

// KeyTypeValue is an OptionalValue allowing specification of the -t argument
// to ssh-keygen.
type KeyTypeValue struct {
	specified bool
	keyType   string
}

func (ktv *KeyTypeValue) String() string {
	return ktv.keyType
}

func (ktv *KeyTypeValue) Set(s string) error {
	if len(s) > 0 {
		ktv.keyType = s
		ktv.specified = true
	}
	return nil
}

func (ktv *KeyTypeValue) Type() string {
	return "string"
}

func (ktv *KeyTypeValue) Specified() bool {
	return ktv.specified
}

// KeyGen generates a new keypair with ssh-keygen, optionally overriding the
// default type and size. Each generated keypair is written to a new unique
// subdirectory of tmpfsPath, which should point to a tmpfs mount as the
// private key is not encrypted.
func KeyGen(keyBits, keyType OptionalValue, tmpfsPath string) (privateKeyPath string, privateKey []byte, publicKey PublicKey, err error) {
	tempDir, err := ioutil.TempDir(tmpfsPath, "..weave-keygen")
	if err != nil {
		return "", nil, PublicKey{}, err
	}

	privateKeyPath = path.Join(tempDir, "identity")
	args := []string{"-q", "-N", "", "-f", privateKeyPath}
	if keyBits.Specified() {
		args = append(args, "-b", keyBits.String())
	}
	if keyType.Specified() {
		args = append(args, "-t", keyType.String())
	}

	cmd := exec.Command("ssh-keygen", args...)
	if err := cmd.Run(); err != nil {
		return "", nil, PublicKey{}, err
	}

	privateKey, err = ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return "", nil, PublicKey{}, err
	}

	publicKey, err = ExtractPublicKey(privateKeyPath)
	if err != nil {
		return "", nil, PublicKey{}, err
	}

	return privateKeyPath, privateKey, publicKey, nil
}

type Fingerprint struct {
	Hash      string `json:"hash"`
	Randomart string `json:"randomart"`
}

var (
	fieldRegexp  = regexp.MustCompile(`^([[:digit:]]+) ([^:]+):([^ ]+) (.*?) \(([^)]+)\)$`)
	captureCount = 6
)

// Fingerprint extracts and returns the hash and randomart of the public key
// associated with the specified private key.
func ExtractFingerprint(privateKeyPath, hashAlgo string) (Fingerprint, error) {
	output, err := exec.Command("ssh-keygen", "-l", "-v", "-E", hashAlgo, "-f", privateKeyPath).Output()
	if err != nil {
		return Fingerprint{}, err
	}

	i := bytes.IndexByte(output, '\n')
	if i == -1 {
		return Fingerprint{}, fmt.Errorf("could not parse fingerprint")
	}

	fields := fieldRegexp.FindSubmatch(output[:i])
	if len(fields) != captureCount {
		return Fingerprint{}, fmt.Errorf("could not parse fingerprint")
	}

	return Fingerprint{
		Hash:      string(fields[3]),
		Randomart: string(output[i+1:]),
	}, nil
}

type PublicKey struct {
	Key          string                 `json:"key"`
	Fingerprints map[string]Fingerprint `json:"fingerprints"`
}

// ExtractPublicKey extracts and returns the public key from the specified
// private key, along with its fingerprint hashes.
func ExtractPublicKey(privateKeyPath string) (PublicKey, error) {
	keyBytes, err := exec.Command("ssh-keygen", "-y", "-f", privateKeyPath).CombinedOutput()
	if err != nil {
		return PublicKey{}, errors.New(string(keyBytes))
	}

	md5Print, err := ExtractFingerprint(privateKeyPath, "md5")
	if err != nil {
		return PublicKey{}, err
	}

	sha256Print, err := ExtractFingerprint(privateKeyPath, "sha256")
	if err != nil {
		return PublicKey{}, err
	}

	return PublicKey{
		Key: string(keyBytes),
		Fingerprints: map[string]Fingerprint{
			"md5":    md5Print,
			"sha256": sha256Print,
		},
	}, nil
}
