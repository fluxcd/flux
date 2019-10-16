# How to Contribute

Flux is [Apache 2.0 licensed](LICENSE) and accepts contributions via GitHub
pull requests. This document outlines some of the conventions on development
workflow, commit message formatting, contact points and other resources to make
it easier to get your contribution accepted.

We gratefully welcome improvements to issues and documentation as well as to code.

## Working on issues

If you like Flux and want to get involved in the project, a great way to get started
is reviewing our [blocked-needs-validation](https://github.com/fluxcd/flux/issues?q=is%3Aissue+is%3Aopen+label%3Ablocked-needs-validation) issues.

The idea here is that new issues are confirmed, which might require asking
for more information, testing with a fresh Flux environment. Once confirmed,
the `blocked-needs-validation` label is removed, and the issue can be worked
on.

To set up Flux to test things, there's documentation about setting up a
[standalone install](docs/tutorials/get-started.md) and a [Helm
install](docs/tutorials/get-started-helm.md), which might be helpful.

Please talk to us on Slack, if you should get stuck anywhere. We appreciate
any help and look forward to talking to you soon!

## Certificate of Origin

By contributing to this project you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. No action from you is required, but it's a good idea to see the
[DCO](DCO) file for details before you start contributing code to Flux.

## Communications

The project uses Slack: To join the conversation, simply join the
[CNCF](https://slack.cncf.io/) Slack workspace and use the
[#flux](https://cloud-native.slack.com/messages/flux/) channel.

The Flux developers use a mailing list to discuss development as well.
Simply subscribe to [flux-dev on cncf.io](https://lists.cncf.io/g/cncf-flux-dev)
to join the conversation (this will also add an invitation to your
Google calendar for our [Flux
meeting](https://docs.google.com/document/d/1l_M0om0qUEN_NNiGgpqJ2tvsF2iioHkaARDeh6b70B0/edit#)).

## Getting Started

- Fork the repository on GitHub
- Read the [README](README.md#get-started-with-flux) for getting started as
  a user and learn how/where to ask for help
- If you want to contribute as a developer, continue reading this document for further instructions
- If you are a new contributor, the following two issue labels might be
  interesting to you:
  - [size/small](https://github.com/fluxcd/flux/issues?q=is%3Aissue+is%3Aopen+label%3Asize%2Fsmall)
  - [help wanted](https://github.com/fluxcd/flux/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)
- If you have questions, concerns, get stuck or need a hand, let us know
  on the Slack channel. We are happy to help and look forward to having
  you part of the team. No matter in which capacity.
- Play with the project, submit bugs, submit pull requests!

## Contribution workflow

This is a rough outline of how to prepare a contribution:

- Create a topic branch from where you want to base your work (usually branched from master).
- Make commits of logical units.
- Make sure your commit messages are in the proper format (see below).
- Push your changes to a topic branch in your fork of the repository.
- If you changed code:
  - add automated tests to cover your changes
- Submit a pull request to the original repository.

### How to build and run the project

Refer to the [building doc](docs/contributing/building.md) to find out how to build from
source.

Refer to the [Get Started Developing](docs/contributing/get-started-developing.md) guide for a walkthrough on developing Flux locally.

### How to run the test suite

You can run the unit tests by simply doing

```bash
make test
```

## Acceptance policy

These things will make a PR more likely to be accepted:

- a well-described requirement
- tests for new code
- tests for old code!
- new code and tests follow the conventions in old code and tests
- a good commit message (see below)
- All code must abide [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Names should abide [What's in a name](https://talks.golang.org/2014/names.slide#1)
- Code must build on both Linux and Darwin, via plain `go build`
- Code should have appropriate test coverage and tests should be written
  to work with `go test`

In general, we will merge a PR once one maintainer has endorsed it.
For substantial changes, more people may become involved, and you might
get asked to resubmit the PR or divide the changes into more than one PR.

### Format of the Commit Message

For Flux we prefer the following rules for good commit messages:

- Limit the subject to 50 characters and write as the continuation
  of the sentence "If applied, this commit will ..."
- Explain what and why in the body, if more than a trivial change;
  wrap it at 72 characters.

The [following article](https://chris.beams.io/posts/git-commit/#seven-rules)
has some more helpful advice on documenting your work.
