package http

const (
	ListServices    = "ListServices"
	ListImages      = "ListImages"
	UpdateManifests = "UpdateManifests"
	JobStatus       = "JobStatus"
	SyncStatus      = "SyncStatus"
	Export          = "Export"
	GitRepoConfig   = "GitRepoConfig"

	UpdateImages           = "UpdateImages"
	UpdatePolicies         = "UpdatePolicies"
	GetPublicSSHKey        = "GetPublicSSHKey"
	RegeneratePublicSSHKey = "RegeneratePublicSSHKey"
)

var (
	RegisterDaemonV6 = "RegisterDaemonV6"
	RegisterDaemonV7 = "RegisterDaemonV7"
	RegisterDaemonV8 = "RegisterDaemonV8"
	RegisterDaemonV9 = "RegisterDaemonV9"
	LogEvent         = "LogEvent"
)
