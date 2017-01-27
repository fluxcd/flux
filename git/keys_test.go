package git

import (
	"strings"
	"testing"
)

func TestKey_Generate(t *testing.T) {
	keygen := NewKeyGenerator()
	priv, err := keygen.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(priv) == 0 {
		t.Fatal("Length should definitely not be zero")
	}
	if !strings.Contains(string(priv), "-----BEGIN RSA PRIVATE KEY-----") {
		t.Fatal("should be priv type", string(priv))
	}
}
