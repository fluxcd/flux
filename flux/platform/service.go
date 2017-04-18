package platform

// Service describes a platform service, generally a floating IP with one or
// more exposed ports that map to a load-balanced pool of instances. Eventually
// this type will generalize to something of a lowest-common-denominator for all
// supported platforms, but right now it looks a lot like a Kubernetes service.
type Service struct {
	Name     string
	IP       string
	Metadata map[string]string // a grab bag of goodies, likely platform-specific
	Status   string            // A status summary for display
}
