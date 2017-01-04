package metrics

/*
Labels and so on for metrics used in Flux.
*/

const (
	LabelInstanceID = "instance_id"
	LabelMethod     = "method"
	LabelNamespace  = "namespace"
	LabelSuccess    = "success"

	// Labels for release metrics
	LabelAction      = "action"
	LabelReleaseType = "release_type"
	LabelStage       = "stage"
)
