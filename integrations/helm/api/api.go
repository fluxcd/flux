package api

// Server is the interface that must be satisfied in order to serve
// HTTP API requests.
type Server interface {
	SyncMirrors()
}
