package github

import (
	"crypto/sha256"
	"fmt"

	"github.com/weaveworks/flux/remote"
)

func MakeGitSubject(platform remote.Platform) (string, error) {
	repoConfig, err := platform.GitRepoConfig(false)
	if err != nil {
		return "", err
	}
	r := repoConfig.Remote
	return GitConfigHash(r.URL, r.Branch), nil
}

func GitConfigHash(url, branch string) string {
	s := fmt.Sprintf("%s:%s", url, branch)
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}
