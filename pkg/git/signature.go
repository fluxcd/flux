package git

// Signature holds information about a GPG signature.
type Signature struct {
	Key    string
	Status string
}

// Valid returns true if the signature is _G_ood (valid).
// https://github.com/git/git/blob/56d268bafff7538f82c01d3c9c07bdc54b2993b1/Documentation/pretty-formats.txt#L146-L153
func (s *Signature) Valid() bool {
	return s.Status == "G"
}
