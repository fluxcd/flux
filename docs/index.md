## Flux v1 is in Maintenance Mode

Please [upgrade to Flux v2](https://toolkit.fluxcd.io/guides/flux-v1-migration/) as soon as possible. [Flux v1 is in Maintenance Mode](faq/#migrate-to-flux-v2)

# Flux documentation

![](_files/flux-cd-diagram.png)

Flux is a tool that automatically ensures that the state of a cluster matches
the config in git. It uses [an operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
in the cluster to trigger deployments inside Kubernetes, which means you don't
need a separate CD tool. It monitors all relevant image repositories, detects
new images, triggers deployments and updates the desired running configuration
based on that (and a configurable policy).

The benefits are: you don't need to grant your CI access to the cluster, every
change is atomic and transactional, git has your audit log. Each transaction
either fails or succeeds cleanly. You're entirely code centric and don't need
new infrastructure.

## Get started

With the following tutorials:

- [Get started with Flux](tutorials/get-started.md)
- [Get started with Flux using Helm](tutorials/get-started-helm.md)

Making use of Helm charts in your cluster? Combine Flux with the [Helm
Operator](https://github.com/fluxcd/helm-operator) to declaratively manage chart
releases using `HelmRelease` custom resources.

For progressive delivery patterns like Canary Releases, A/B Testing and Blue/Green,
Flux can be used together with [Flagger](https://flagger.app).

## Getting help

If you have any questions about Flux and continuous delivery:

- Invite yourself to the <a href="https://slack.cncf.io" target="_blank">CNCF community</a>
  slack and ask a question on the [#flux](https://cloud-native.slack.com/messages/flux/)
  channel.
- To be part of the conversation about Flux's development, join the
  [flux-dev mailing list](https://lists.cncf.io/g/cncf-flux-dev).
- [File an issue.](https://github.com/fluxcd/flux/issues/new/choose)

Your feedback is always welcome!

