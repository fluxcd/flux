package ssh

// KeyRing is an abstraction providing access to a managed SSH key pair. Whilst
// the public half is available in byte form, the private half is left on the
// filesystem to avoid memory management issues.
type KeyRing interface {
	KeyPair() (publicKey PublicKey, privateKeyPath string)
	Regenerate() error
}

type sshKeyRing struct{}

// NewNopSSHKeyRing returns a KeyRing that doesn't do anything.
// It is meant for local development purposes when running fluxd outside a Kubernetes container.
func NewNopSSHKeyRing() KeyRing {
	return &sshKeyRing{}
}

func (skr *sshKeyRing) KeyPair() (PublicKey, string) {
	return PublicKey{}, ""
}

func (skr *sshKeyRing) Regenerate() error {
	return nil
}
