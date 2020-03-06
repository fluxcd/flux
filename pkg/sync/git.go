package sync

import (
	"context"
	"strings"

	"github.com/fluxcd/flux/pkg/git"
)

// GitTagSyncProvider is the mechanism by which a Git tag is used to keep track of the current point fluxd has synced to.
type GitTagSyncProvider struct {
	repo          *git.Repo
	syncTag       string
	signingKey    string
	verifyTagMode VerifySignaturesMode
	config        git.Config
}

// NewGitTagSyncProvider creates a new git tag sync provider.
func NewGitTagSyncProvider(
	repo *git.Repo,
	syncTag string,
	signingKey string,
	verifyTagMode VerifySignaturesMode,
	config git.Config,
) (GitTagSyncProvider, error) {
	return GitTagSyncProvider{
		repo:          repo,
		syncTag:       syncTag,
		signingKey:    signingKey,
		verifyTagMode: verifyTagMode,
		config:        config,
	}, nil
}

func (p GitTagSyncProvider) String() string {
	return "tag " + p.syncTag
}

// GetRevision returns the revision of the git commit where the flux sync tag is currently positioned.
func (p GitTagSyncProvider) GetRevision(ctx context.Context) (string, error) {
	rev, err := p.repo.Revision(ctx, p.syncTag)
	if isUnknownRevision(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if p.verifyTagMode != VerifySignaturesModeNone {
		if _, err := p.repo.VerifyTag(ctx, p.syncTag); err != nil {
			// if the revision wasn't found, don't treat this as an
			// error -- but don't supply a revision, either.
			if strings.Contains(err.Error(), "not found.") {
				return "", nil
			}
			return "", err
		}
	}
	return rev, nil
}

// UpdateMarker moves the sync tag in the upstream repo.
func (p GitTagSyncProvider) UpdateMarker(ctx context.Context, revision string) error {
	checkout, err := p.repo.Clone(ctx, p.config)
	if err != nil {
		return err
	}
	defer checkout.Clean()
	return checkout.MoveTagAndPush(ctx, git.TagAction{
		Tag:        p.syncTag,
		Revision:   revision,
		Message:    "Sync pointer",
		SigningKey: p.signingKey,
	})
}

// DeleteMarker removes the Git Tag used for syncing.
func (p GitTagSyncProvider) DeleteMarker(ctx context.Context) error {
	return p.repo.DeleteTag(ctx, p.syncTag)
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}
