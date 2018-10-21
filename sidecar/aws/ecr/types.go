package ecr

// Contants that will be used by the ecr sidecar and flux daemon.
const (
	SidecarAWSPort = "3031"
	SidecarAWSPath = "/awsauth"
	SidecarAWSURL  = "http://localhost:" + SidecarAWSPort + SidecarAWSPath
)
