package platform

// A Container represents a container specification in a pod. Name identifies it
// within the pod, and Image says which image it's configured to run.
type Container struct {
	Name  string
	Image string
}
