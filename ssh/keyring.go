package ssh

// KeyRing is an abstraction providing access to a managed SSH key pair. Whilst
// the public half is available in byte form, the private half is left on the
// filesystem to avoid memory management issues.
type KeyRing interface {
	KeyPair() (publicKey PublicKey, privateKeyPath string)
	Regenerate() error
}
