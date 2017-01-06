package release

import "testing"

func TestReleaser_NoUpdateImage(t *testing.T) {
	// It should not check for new images
	// It should not update any new definitions
	// It should not commit/push
	// It should apply only specified services to kubernetes
	t.Error("TODO")
}

func TestReleaser_DryRun(t *testing.T) {
	// It should check for new images
	// It should update any new definitions
	// It should not commit/push
	// It should not apply to kubernetes
	t.Error("TODO")
}

func TestReleaser_ExactImageExactService(t *testing.T) {
	// It should not check for new images
	// It should update any new definitions
	// It should commit/push
	// It should apply only specified services to kubernetes
	t.Error("TODO")
}

func TestReleaser_LatestImagesExactService(t *testing.T) {
	// It should check for new images
	// It should update any new definitions
	// It should commit/push
	// It should apply only specified services to kubernetes
	t.Error("TODO")
}

func TestReleaser_LatestImagesAllServices(t *testing.T) {
	// It should check for new images
	// It should update only changed definitions
	// It should commit/push
	// It should apply only changed services to kubernetes
	t.Error("TODO")
}

func TestReleaser_DeployingANonRunningService(t *testing.T) {
	// It should work fine
	t.Error("TODO")
}

func TestReleaser_NoDefinitionsForAService(t *testing.T) {
	// It should abort and error
	t.Error("TODO")
}

func TestReleaser_MultipleDefinitionsForAService(t *testing.T) {
	// It should abort and error
	t.Error("TODO")
}

func TestReleaser_Notifications(t *testing.T) {
	// It should check for new images
	// It should update only changed definitions
	// It should commit/push
	// It should apply only changed services to kubernetes
	t.Error("TODO")
}

func TestReleaser_Notifications_WithFailures(t *testing.T) {
	// It should not send notification when there was a failure during plan phase
	// It should still send notification when there was a failure during execute. phase
	//
	// TODO: Is this even the right logic? We want a notification if a
	// background thing fails, but not if it is a user, unless it failed while
	// they were meddling with the live cluster. It is difficult to encapsulate
	// cleanly.
	t.Error("TODO")
}
