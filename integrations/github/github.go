package github

import (
	"fmt"
	gh "github.com/google/go-github/github"
	"github.com/weaveworks/flux/http/httperror"
	"golang.org/x/oauth2"
	"net/http"
)

var (
	deployKeyName   = "flux-generated"
	webhookName     = "flux-generated"
	webhookEvents   = []string{"push"}
	errUnauthorized = httperror.APIError{
		Body: "Unable to list deploy keys. Permission deined. Check user token.",
	}
	errNotFound = httperror.APIError{
		Body: "Cannot find owner or repository. Check spelling.",
	}
	errGeneric = httperror.APIError{
		Body: "Unable to perform GH action. Check error message.",
	}
)

type github struct {
	client *gh.Client
}

// NewGithubClient instantiates a GH client from a provided OAuth token.
func NewGithubClient(token string) *github {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return &github{
		client: gh.NewClient(tc),
	}
}

// InsertWebhook will create a new webhook for the given owner,
// repo, token using the endpoint specified.
// If a webhook already exists with that endpoint it will be replaced.
func (g *github) InsertWebhook(ownerName string, repoName string, endpoint string) error {
	// Get list of hooks
	hooks, resp, err := g.client.Repositories.ListHooks(ownerName, repoName, nil)
	if err != nil {
		return parseError(resp, err)
	}
	for _, h := range hooks {
		// If key already exists, delete
		if *h.URL == endpoint {
			resp, err := g.client.Repositories.DeleteHook(ownerName, repoName, *h.ID)
			if err != nil {
				return parseError(resp, err)
			}
			break
		}
	}

	// Create new hook
	var active = true
	_, resp, err = g.client.Repositories.CreateHook(ownerName, repoName, &gh.Hook{
		Name:   &webhookName,
		URL:    &endpoint,
		Events: webhookEvents,
		Active: &active,
	})
	if err != nil {
		return parseError(resp, err)
	}
	return nil
}

// DeleteWebhook will delete a webhook for the given owner,
// repo, token matching the endpoint specified.
// If a webhook does not exist this is a noop.
func (g *github) DeleteWebhook(ownerName string, repoName string, endpoint string) error {
	// Get list of hooks
	hooks, resp, err := g.client.Repositories.ListHooks(ownerName, repoName, nil)
	if err != nil {
		return parseError(resp, err)
	}
	for _, h := range hooks {
		// If key already exists, delete
		if *h.URL == endpoint {
			resp, err := g.client.Repositories.DeleteHook(ownerName, repoName, *h.ID)
			if err != nil {
				return parseError(resp, err)
			}
			break
		}
	}
	return nil
}

// InsertDeployKey will create a new deploy key for the given owner,
// repo, token using the key deployKey.
// If a key already exists with that name it will be deleted.
func (g *github) InsertDeployKey(ownerName string, repoName string, deployKey string) error {
	// Get list of keys
	keys, resp, err := g.client.Repositories.ListKeys(ownerName, repoName, nil)
	if err != nil {
		return parseError(resp, err)
	}
	for _, k := range keys {
		// If key already exists, delete
		if *k.Title == deployKeyName {
			resp, err := g.client.Repositories.DeleteKey(ownerName, repoName, *k.ID)
			if err != nil {
				return parseError(resp, err)
			}
			break
		}
	}

	// Create new key
	key := gh.Key{
		Title: &deployKeyName,
		Key:   &deployKey,
	}
	_, resp, err = g.client.Repositories.CreateKey(ownerName, repoName, &key)
	if err != nil {
		return parseError(resp, err)
	}
	return nil
}

func populateError(err httperror.APIError, resp *gh.Response) *httperror.APIError {
	err.StatusCode = resp.StatusCode
	err.Status = resp.Status
	return &err
}

func parseError(resp *gh.Response, err error) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return populateError(errUnauthorized, resp)
	case http.StatusNotFound:
		return populateError(errNotFound, resp)
	default:
		e := populateError(errGeneric, resp)
		e.Body = fmt.Sprintf("%s - %s", e.Body, err.Error())
		return e
	}
}
