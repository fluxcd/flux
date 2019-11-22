package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/google/go-github/v28/github"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

const (
	gitHubOrg  = "fluxcd"
	gitHubRepo = "flux"
)

// Generate a changelog since
func main() {
	from := pflag.String("from", "", "git revision use as tge release starting point (e.g. 1.15.0), if none is provided the tag of the latest release is used")
	pflag.Parse()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("No token provided")
	}
	to := "master"

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	if *from == "" {
		// get the tag of the latest release
		latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, gitHubOrg, gitHubRepo)
		if err != nil {
			log.Fatalf("cannot obtain latest release: %s", err)
		}
		*from = latestRelease.GetTagName()
	}

	fromSHA1, _, err := client.Repositories.GetCommitSHA1(ctx, gitHubOrg, gitHubRepo, *from, "")
	if err != nil {
		log.Fatalf("cannot obtain SHA1 of %q: %s", from)
	}
	toSHA1, _, err := client.Repositories.GetCommitSHA1(ctx, gitHubOrg, gitHubRepo, to, "")
	if err != nil {
		log.Fatalf("cannot obtain SHA1 of %q: %s", from)
	}

	// Make sure that the "from" revision is behing "to" and obtain the number of commits, for the progess bar
	comparison, _, err := client.Repositories.CompareCommits(ctx, gitHubOrg, gitHubRepo, toSHA1, fromSHA1)
	if err != nil {
		log.Fatalf("cannot compare commits from %q and to %q: %s", from, to, err)
	}
	if comparison.GetStatus() != "behind" {
		log.Fatalf("'from' revision (%s) is not behind 'to' revision (%s)", *from, to)
	}
	commitCandidateCount := comparison.GetBehindBy()
	progressBar := pb.New(commitCandidateCount)
	progressBar.SetTemplateString(`Processing commit {{counters . }} {{bar . }} {{percent . }} {{etime . "%s"}}`)
	progressBar.Start()

	commitsListOptions := &github.CommitsListOptions{
		SHA: toSHA1,
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 50,
		},
	}

	// Store pull requests in visiting order, preventing duplicates
	prs := []*github.PullRequest{}
	visitedPRs := map[int]struct{}{}
listCommits:
	for {
		commits, response, err := client.Repositories.ListCommits(ctx, gitHubOrg, gitHubRepo, commitsListOptions)
		if err != nil {
			log.Fatalf("cannot list commits: %s", err)
		}
		for _, commit := range commits {
			progressBar.Increment()
			if commit.GetSHA() == fromSHA1 {
				// We have gone through all the commits
				break listCommits
			}
			pullRequestListOptions := &github.PullRequestListOptions{
				Base: "master", // we are only interested in pull requests reaching master
			}
			prsForCommit, _, err := client.PullRequests.ListPullRequestsWithCommit(ctx, gitHubOrg, gitHubRepo, commit.GetSHA(), pullRequestListOptions)
			if err != nil {
				log.Fatalf("cannot list pull requests for commit %q: %s", commit.GetSHA(), err)
			}
			for _, pr := range prsForCommit {
				repoFullName := gitHubOrg + "/" + gitHubRepo
				if pr.GetBase().GetRepo().GetFullName() != repoFullName {
					// This commit was previously merged to master, but in a different repository
					// and ended up in our master branch.
					continue
				}
				if _, ok := visitedPRs[pr.GetNumber()]; ok {
					continue
				}
				visitedPRs[pr.GetNumber()] = struct{}{}
				prs = append(prs, pr)
			}
		}
		if commitsListOptions.Page == response.LastPage {
			log.Printf("warning: run out of commits page without finding base commit (%s)", fromSHA1)
			break
		}
		commitsListOptions.Page = response.NextPage
	}
	progressBar.Finish()

	contributors := map[string]struct{}{}
	uncategorizedPRs := []*github.PullRequest{}
	documentationPRs := []*github.PullRequest{}
	fixPRs := []*github.PullRequest{}
prLoop:
	for _, pr := range prs {
		contributors[pr.GetUser().GetLogin()] = struct{}{}
		for _, label := range pr.Labels {
			if label.GetName() == "helm-chart" {
				// Helm chart pull requests should be excluded since charts are released separately
				continue prLoop
			}
			if label.GetName() == "docs" {
				documentationPRs = append(documentationPRs, pr)
				continue prLoop
			}
			if label.GetName() == "bug" {
				fixPRs = append(fixPRs, pr)
				continue prLoop
			}
		}
		// TODO: try to find whether the PR automatically closed an issue
		//       and use the labels of that issue to categorize the PR
		uncategorizedPRs = append(uncategorizedPRs, pr)
	}

	sortedContributors := []string{}
	for c := range contributors {
		sortedContributors = append(sortedContributors, c)
	}
	sort.Strings(sortedContributors)

	printPRItems := func(prs []*github.PullRequest) {
		for _, pr := range prs {
			fmt.Printf("- %s [%s/%s#%d][]\n",
				pr.GetTitle(), gitHubOrg, gitHubRepo, pr.GetNumber())
		}
	}

	fmt.Printf("## <MajorVersion>.<MinorVersion>.<PatchVersion> (%s)\n", time.Now().Format("2006-01-02"))
	fmt.Println()
	fmt.Println("<Add release description here>")
	if len(uncategorizedPRs) > 0 {
		fmt.Println()
		fmt.Println("### Uncategorized")
		fmt.Println()
		printPRItems(uncategorizedPRs)
	}
	if len(fixPRs) > 0 {
		fmt.Println()
		fmt.Println("### Fixes")
		fmt.Println()
		printPRItems(fixPRs)
	}
	if len(documentationPRs) > 0 {
		fmt.Println()
		fmt.Println("### Documentation")
		fmt.Println()
		printPRItems(documentationPRs)
	}
	if len(sortedContributors) > 0 {
		fmt.Println()
		fmt.Println("### Thanks")
		var renderedContributors string
		for i := 0; i < len(sortedContributors); i++ {
			if i > 0 {
				if i == len(sortedContributors)-1 {
					renderedContributors += " and "
				} else {
					renderedContributors += ", "
				}
			}
			renderedContributors += "@" + sortedContributors[i]
		}
		fmt.Println()
		fmt.Printf("Thanks to %s for their contributions to this release.\n",
			renderedContributors)
	}
	// Print reference links
	if len(prs) > 0 {
		fmt.Println()
		sortedPRNums := []int{}
		for _, pr := range prs {
			sortedPRNums = append(sortedPRNums, pr.GetNumber())
		}
		sort.Ints(sortedPRNums)
		// print in reverse order
		for i := len(sortedPRNums) - 1; i >= 0; i-- {
			prNum := sortedPRNums[i]
			fmt.Printf("[%s/%s#%d]: https://github.com/%s/%s/pull/%d\n",
				gitHubOrg, gitHubRepo, prNum, gitHubOrg, gitHubRepo, prNum)
		}
	}
}
