package http

const (
	// Formerly Upstream methods, now (in v11) included in server API
	Ping    = "Ping"
	Version = "Version"
	Notify  = "Notify"

	ListServices            = "ListServices"
	ListServicesWithOptions = "ListServicesWithOptions"
	ListImages              = "ListImages"
	ListImagesWithOptions   = "ListImagesWithOptions"
	UpdateManifests         = "UpdateManifests"
	JobStatus               = "JobStatus"
	SyncStatus              = "SyncStatus"
	Export                  = "Export"
	GitRepoConfig           = "GitRepoConfig"

	UpdateImages           = "UpdateImages"
	UpdatePolicies         = "UpdatePolicies"
	GetPublicSSHKey        = "GetPublicSSHKey"
	RegeneratePublicSSHKey = "RegeneratePublicSSHKey"
)

// This is part of the API -- but it's the outward-facing (or service
// provider) API, rather than the flux API.
const (
	LogEvent = "LogEvent"
)

// The RegisterDaemonX routes should move to weaveworks/flux-adapter
// once we remove `--connect`, since they are pertinent only to making
// an RPC relay connection.
const (
	RegisterDaemonV6  = "RegisterDaemonV6"
	RegisterDaemonV7  = "RegisterDaemonV7"
	RegisterDaemonV8  = "RegisterDaemonV8"
	RegisterDaemonV9  = "RegisterDaemonV9"
	RegisterDaemonV10 = "RegisterDaemonV10"
	RegisterDaemonV11 = "RegisterDaemonV11"
)
