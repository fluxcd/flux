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
	expected := map[string]string{
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}

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
	expected := map[string]string{
		"user.name":                    "Jane Doe",
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
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
	input := map[string]string{
		"user.name":                    "Jane Doe",
		"user.email":                   "jd@j.d",
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	expected := "Jane Doe <jd@j.d>"
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatal("author did not match expected value")
	}
}

func TestGetCommitAuthor_OnlyName(t *testing.T) {
	input := map[string]string{
		"user.name":                    "Jane Doe",
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	expected := "Jane Doe"
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatal("author did not match expected value")
	}
}

func TestGetCommitAuthor_OnlyEmail(t *testing.T) {
	input := map[string]string{
		"user.email":                   "jd@j.d",
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	expected := "jd@j.d"
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatal("author did not match expected value")
	}
}

func TestGetCommitAuthor_NoNameNoEmail(t *testing.T) {
	input := map[string]string{
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	expected := ""
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatal("author did not match expected value")
	}
}

func TestGetCommitAuthor_NameAndEmptyEmail(t *testing.T) {
	input := map[string]string{
		"user.name":                    "Jane Doe",
		"user.email":                   "",
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	expected := "Jane Doe"
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatal("author did not match expected value")
	}
}

func TestGetCommitAuthor_EmailAndEmptyName(t *testing.T) {
	input := map[string]string{
		"user.name":                    "",
		"user.email":                   "jd@j.d",
		"push.default":                 "simple",
		"merge.conflictstyle":          "diff3",
		"pull.ff":                      "only",
		"core.repositoryformatversion": "0",
		"core.filemode":                "true",
		"core.bare":                    "false",
	}
	expected := "jd@j.d"
	author := getCommitAuthor(input)
	if author != expected {
		t.Fatal("author did not match expected value")
	}
}
