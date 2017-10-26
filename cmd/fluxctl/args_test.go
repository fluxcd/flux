package main

import (
	"reflect"
	"testing"
)

func TestUserGitconfigMap_EmptyString(t *testing.T) {
	d := ""
	userGitconfigInfo := userGitconfigMap(d)
	if len(userGitconfigInfo) != 0 {
		t.Fatal("expected map with no keys")
	}
}

func TestUserGitconfigMap(t *testing.T) {
	d := `push.default=simple
	merge.conflictstyle=diff3
	pull.ff=only
	core.repositoryformatversion=0
	core.filemode=true
	core.bare=false`
	expected := gitConfigMap(nil)

	userGitconfigInfo := userGitconfigMap(d)
	if len(userGitconfigInfo) != 6 {
		t.Fatal("got map with unexpected number of keys")
	}
	if !reflect.DeepEqual(userGitconfigInfo, expected) {
		t.Fatal("result does not match expected structure")
	}
}

func TestUserGitconfigMap_WithEmptyLines(t *testing.T) {
	d := `
	user.name=Jane Doe
	push.default=simple
	merge.conflictstyle=diff3
	pull.ff=only

	core.repositoryformatversion=0
	core.filemode=true
	core.bare=false

	`
	expected := gitConfigMap(map[string]string{
		"user.name": "Jane Doe",
	})
	userGitconfigInfo := userGitconfigMap(d)

	if len(userGitconfigInfo) != 7 {
		t.Fatal("got map with unexpected number of keys")
	}
	if !reflect.DeepEqual(userGitconfigInfo, expected) {
		t.Fatal("result does not match expected structure")
	}
}

func TestUserGitconfigMap_WithNoKeys(t *testing.T) {
	d := `
	`
	expected := make(map[string]string)

	userGitconfigInfo := userGitconfigMap(d)
	if len(userGitconfigInfo) != 0 {
		t.Fatal("expected map with no keys")
	}
	if !reflect.DeepEqual(userGitconfigInfo, expected) {
		t.Fatal("result does not match expected structure")
	}
}

func TestGetCommitAuthor_BothNameAndEmail(t *testing.T) {
	input := gitConfigMap(map[string]string{
		"user.name":  "Jane Doe",
		"user.email": "jd@j.d",
	})
	checkAuthor(t, input, "Jane Doe <jd@j.d>")
}

func TestGetCommitAuthor_OnlyName(t *testing.T) {
	input := gitConfigMap(map[string]string{
		"user.name": "Jane Doe",
	})
	checkAuthor(t, input, "Jane Doe")
}

func TestGetCommitAuthor_OnlyEmail(t *testing.T) {
	input := gitConfigMap(map[string]string{
		"user.email": "jd@j.d",
	})
	checkAuthor(t, input, "jd@j.d")
}

func TestGetCommitAuthor_NoNameNoEmail(t *testing.T) {
	input := gitConfigMap(nil)
	checkAuthor(t, input, "")
}

func TestGetCommitAuthor_NameAndEmptyEmail(t *testing.T) {
	input := gitConfigMap(map[string]string{
		"user.name":  "Jane Doe",
		"user.email": "",
	})
	checkAuthor(t, input, "Jane Doe")
}

func TestGetCommitAuthor_EmailAndEmptyName(t *testing.T) {
	input := gitConfigMap(map[string]string{
		"user.name":  "",
		"user.email": "jd@j.d",
	})
	checkAuthor(t, input, "jd@j.d")
}

func checkAuthor(t *testing.T, input map[string]string, expected string) {
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatalf("author %q does not match expected value %q", author, expected)
	}
}

func gitConfigMap(input map[string]string) map[string]string {
	res := map[string]string{
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	for k, v := range input {
		res[k] = v
	}
	return res
}
