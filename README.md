# Flux

We believe that environments should be entirely version
controlled. This is an anti-fragile measure to ensure stability
through visibility. If anything fails, you can simply reapply the
current state of the repository.

Flux is a tool that automatically ensures that the state of a cluster
matches what is specified in version control (along with a few
[extra features](/site/how-it-works.md)).

It is most useful when used as a deployment tool at the end of a
Continuous Delivery pipeline. Flux will make sure that your new
container images and config changes are propagated to the cluster.

[![CircleCI](https://circleci.com/gh/weaveworks/flux.svg?style=svg)](https://circleci.com/gh/weaveworks/flux)
[![GoDoc](https://godoc.org/github.com/weaveworks/flux?status.svg)](https://godoc.org/github.com/weaveworks/flux)

Get started by browsing through the documentation below.

[Introduction to Flux](/site/introduction.md)

[FAQ](/site/faq.md)

[How it works](/site/how-it-works.md)

[Installing Flux](/site/installing.md)

[Using Flux](/site/using.md)

[Upgrading to Flux v1](/site/upgrading-to-1.0.md)

[Troubleshooting](/site/troubleshooting.md)

## Developer information

[Build documentation](/site/building.md)

[Release documentation](/internal_docs/releasing.md)

### Contribution

Flux follows a typical PR workflow.
All contributions should be made as PRs that satisfy the guidelines below.

### Guidelines

- All code must abide [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Names should abide [What's in a name](https://talks.golang.org/2014/names.slide#1)
- Code must build on both Linux and Darwin, via plain `go build`
- Code should have appropriate test coverage, invoked via plain `go test`

In addition, several mechanical checks are enforced.
See [the lint script](/lint) for details.

## <a name="help"></a>Getting Help

If you have any questions about Flux and continuous delivery:

- Invite yourself to the <a href="https://weaveworks.github.io/community-slack/" target="_blank"> #weave-community </a> slack channel.
- Ask a question on the <a href="https://weave-community.slack.com/messages/general/"> #weave-community</a> slack channel.
- Join the <a href="https://www.meetup.com/pro/Weave/"> Weave User Group </a> and get invited to online talks, hands-on training and meetups in your area.
- Send an email to <a href="mailto:weave-users@weave.works">weave-users@weave.works</a>
- <a href="https://github.com/weaveworks/flux/issues/new">File an issue.</a>

Your feedback is always welcome!
