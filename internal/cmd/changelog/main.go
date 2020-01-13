package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/google/go-github/v28/github"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

const uncategorizedSectionTitle = "Uncategorized"

func getCommitsInRange(ctx context.Context, ghClient *github.Client, gitHubOrg string, gitHubRepo string,
	fromRevision string, toRevision string) ([]github.RepositoryCommit, error) {

	// Make sure that the "from" revision is behind "to" and obtain the commits
	comparison, _, err := ghClient.Repositories.CompareCommits(ctx, gitHubOrg, gitHubRepo, fromRevision, toRevision)
	if err != nil {
		return nil, fmt.Errorf("cannot compare commits from %q and to %q: %s", fromRevision, toRevision, err)
	}
	if comparison.GetStatus() != "ahead" || comparison.GetBehindBy() != 0 {
		return nil, fmt.Errorf("'from' revision (%s) is not behind 'to' revision (%s)", fromRevision, toRevision)
	}
	return comparison.Commits, nil
}

func findPullRequestsInCommits(ctx context.Context, ghClient *github.Client, gitHubOrg string, gitHubRepo string,
	commits []github.RepositoryCommit, commitProcessHook func()) ([]*github.PullRequest, error) {
	// Store pull requests in visiting order, preventing duplicates
	prs := []*github.PullRequest{}
	visitedPRs := map[int]struct{}{}
	for _, commit := range commits {
		commitProcessHook()
		pullRequestListOptions := &github.PullRequestListOptions{}
		// Note: Unfortunately this will return all the PRs whose head branch contains the commit
		// (i.e. not just the PR which incorporates the commit to the base repo).
		// We deal with this by discarding duplicates.
		prsForCommit, _, err := ghClient.PullRequests.ListPullRequestsWithCommit(ctx,
			gitHubOrg, gitHubRepo, commit.GetSHA(), pullRequestListOptions)
		if err != nil {
			log.Fatalf("cannot list pull requests for commit %q: %s", commit.GetSHA(), err)
		}
		for _, pr := range prsForCommit {
			repoFullName := gitHubOrg + "/" + gitHubRepo
			if pr.GetBase().GetRepo().GetFullName() != repoFullName {
				// This commit comes from a PR into a different repository, which we are not interested in
				continue
			}
			if _, ok := visitedPRs[pr.GetNumber()]; ok {
				// don't add duplicates
				continue
			}
			visitedPRs[pr.GetNumber()] = struct{}{}
			prs = append(prs, pr)
		}
	}
	return prs, nil
}

type releaseChangelog struct {
	sections             []changelogSection
	contributorUserNames []string
}

type changelogSection struct {
	title        string
	pullRequests []*github.PullRequest
}

type sectionSpec struct {
	label string
	title string
}

func generateReleaseChangelog(pullRequests []*github.PullRequest,
	exclusionLabels map[string]struct{}, sectionSpecs []sectionSpec) releaseChangelog {

	prsBySectionLabel := map[string][]*github.PullRequest{}
	for _, spec := range sectionSpecs {
		prsBySectionLabel[spec.label] = nil
	}
	uncategorizedPRs := []*github.PullRequest{}
	contributors := map[string]struct{}{}
prLoop:
	for _, pr := range pullRequests {
		sectionLabel := ""
		for _, label := range pr.Labels {
			if _, ok := exclusionLabels[label.GetName()]; ok {
				// Ignore PR since it was tagged with a label we should ignore
				continue prLoop
			}
			if prs, ok := prsBySectionLabel[label.GetName()]; ok {
				// Found pull request from a predefined section
				sectionLabel = label.GetName()
				prsBySectionLabel[sectionLabel] = append(prs, pr)
				break
			}
		}
		if sectionLabel == "" {
			uncategorizedPRs = append(uncategorizedPRs, pr)
		}
		contributors[pr.GetUser().GetLogin()] = struct{}{}
	}

	// Sort sections according to the provided specifications (but starting with the uncategorized one)
	finalSections := []changelogSection{}
	if len(uncategorizedPRs) > 0 {
		uncategorizedSection := changelogSection{uncategorizedSectionTitle, uncategorizedPRs}
		finalSections = append(finalSections, uncategorizedSection)
	}
	for _, ss := range sectionSpecs {
		if len(prsBySectionLabel[ss.label]) > 0 {
			section := changelogSection{ss.title, prsBySectionLabel[ss.label]}
			finalSections = append(finalSections, section)
		}
	}

	// sort contributors alphabetically
	var sortedContributors []string
	for c := range contributors {
		sortedContributors = append(sortedContributors, c)
	}
	sort.Strings(sortedContributors)

	changelog := releaseChangelog{
		sections:             finalSections,
		contributorUserNames: sortedContributors,
	}

	return changelog
}

type sorteablePRs []*github.PullRequest

func (s sorteablePRs) Len() int           { return len(s) }
func (s sorteablePRs) Less(i, j int) bool { return s[i].GetNumber() < s[j].GetNumber() }
func (s sorteablePRs) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func renderReleaseChangelog(out io.Writer, changelog releaseChangelog) {
	var allPullRequests sorteablePRs

	fmt.Fprintf(out, "## <MajorVersion>.<MinorVersion>.<PatchVersion> (%s)\n\n", time.Now().Format("2006-01-02"))
	fmt.Fprintln(out, "<Add release description here>")
	for _, section := range changelog.sections {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "### %s\n\n", section.title)
		for _, pr := range section.pullRequests {
			allPullRequests = append(allPullRequests, pr)
			fmt.Printf("- %s [%s#%d][]\n",
				pr.GetTitle(), pr.GetBase().GetRepo().GetFullName(), pr.GetNumber())
		}
	}

	if len(changelog.contributorUserNames) > 0 {
		contributors := changelog.contributorUserNames
		fmt.Fprintln(out)
		fmt.Fprintln(out, "### Thanks")
		var renderedContributors string
		for i := 0; i < len(contributors); i++ {
			if i > 0 {
				if i == len(contributors)-1 {
					renderedContributors += " and "
				} else {
					renderedContributors += ", "
				}
			}
			renderedContributors += "@" + contributors[i]
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Thanks to %s for their contributions to this release.\n", renderedContributors)
	}

	// Print reference links in reverse order
	sort.Sort(allPullRequests)
	if len(allPullRequests) > 0 {
		fmt.Fprintln(out)
		for i := len(allPullRequests) - 1; i >= 0; i-- {
			repoName := allPullRequests[i].GetBase().GetRepo().GetFullName()
			prNum := allPullRequests[i].GetNumber()
			fmt.Fprintf(out, "[%s#%d]: https://github.com/%s/pull/%d\n", repoName, prNum, repoName, prNum)
		}
	}
}

func main() {
	from := pflag.String("from", "", "git revision to use as the release starting point (e.g. 1.15.0). If none is provided the tag of the latest release is used")
	to := pflag.String("to", "master", "git revision to use as the release ending point")
	gitHubOrg := pflag.String("gh-org", "fluxcd", "GitHub organization of the repository for which to generate the changelog entry")
	gitHubRepo := pflag.String("gh-repo", "flux", "GitHub repository for which to generate the changelog entry")
	excludeLabelStrings := pflag.StringSlice("exclude-labels", []string{"helm-chart"}, "Exclude pull requests tagged with any of these labels")
	sectionSpecStrings := pflag.StringSlice("section-spec", []string{"bug:Fixes", "enhacement:Enhancements", "docs:Documentation"}, "`label:Title` section specifications. `label:Title` indicates to create a section with `Title` in which to include all the pull requests tagged with label `label`")
	pflag.Parse()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("No GitHub token provided, please set the GITHUB_TOKEN env variable")
	}

	var sectionSpecs []sectionSpec
	for _, specString := range *sectionSpecStrings {
		s := strings.Split(specString, ":")
		if len(s) != 2 {
			log.Fatalf("incorrect section spect string %q", specString)
		}
		sectionSpecs = append(sectionSpecs, sectionSpec{label: s[0], title: s[1]})
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	if *from == "" {
		// get the tag of the latest release
		latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, *gitHubOrg, *gitHubRepo)
		if err != nil {
			log.Fatalf("cannot obtain latest release: %s", err)
		}
		*from = latestRelease.GetTagName()
	}

	commits, err := getCommitsInRange(ctx, client, *gitHubOrg, *gitHubRepo, *from, *to)
	if err != nil {
		log.Fatal(err)
	}
	// reverse commits, so that they are processed from newer to older
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	progressBar := pb.New(len(commits))
	progressBar.SetTemplateString(`Processing commit {{counters . }} {{bar . }} {{percent . }} {{etime . "%s"}}`)
	progressBar.Start()
	onCommit := func() { progressBar.Increment() }
	prs, err := findPullRequestsInCommits(ctx, client, *gitHubOrg, *gitHubRepo, commits, onCommit)
	if err != nil {
		log.Fatal(err)
	}
	progressBar.Finish()

	excludeLabels := map[string]struct{}{}
	for _, label := range *excludeLabelStrings {
		excludeLabels[label] = struct{}{}
	}

	changelog := generateReleaseChangelog(prs, excludeLabels, sectionSpecs)

	renderReleaseChangelog(os.Stdout, changelog)
}
