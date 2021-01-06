# Flux

> **On Flux v2** In an announcement in August 2019, the expectation
> was set that the Flux project would integrate the GitOps Engine,
> then being factored out of ArgoCD. Since the result would be
> backward-incompatible, it would require a major version bump: Flux
> v2.
>
> After experimentation and considerable thought, we (the maintainers)
> have found a path to Flux v2 that we think better serves our vision
> of GitOps: the [GitOps Toolkit](https://toolkit.fluxcd.io/). In
> consequence, we do not now plan to integrate GitOps Engine into
> Flux.
>
> :warning: This also means that **[Flux v1 is in maintenance mode](https://github.com/fluxcd/flux/issues/3320)**.

We believe in GitOps:

- **You declaratively describe the entire desired state of your
  system in git.** This includes the apps, config, dashboards,
  monitoring and everything else.
- **What can be described can be automated.** Use YAMLs to enforce
  conformance of the system. You don't need to run `kubectl`, all changes go
  through git. Use diff tools to detect divergence between observed and
  desired state and get notifications.
- **You push code not containers.** Everything is controlled through
  pull requests. There is no learning curve for new devs, they just use
  your standard git PR process. The history in git allows you to recover
  from any snapshot as you have a sequence of transactions. It is much
  more transparent to make operational changes by pull request, e.g.
  fix a production issue via a pull request instead of making changes to
  the running system.

Flux is a tool that automatically ensures that the state of a cluster
matches the config in git. It uses an operator in the cluster to trigger
deployments inside Kubernetes, which means you don't need a separate CD tool.
It monitors all relevant image repositories, detects new images, triggers
deployments and updates the desired running configuration based on that
(and a configurable policy).

The benefits are: you don't need to grant your CI access to the cluster, every
change is atomic and transactional, git has your audit log. Each transaction
either fails or succeeds cleanly. You're entirely code centric and don't need
new infrastructure.

![Deployment Pipeline](docs/_files/flux-cd-diagram.png)

[![CircleCI](https://circleci.com/gh/fluxcd/flux.svg?style=svg)](https://circleci.com/gh/fluxcd/flux)
[![GoDoc](https://godoc.org/github.com/fluxcd/flux?status.svg)](https://godoc.org/github.com/fluxcd/flux)
[![Documentation](https://img.shields.io/badge/latest-documentation-informational)](https://docs.fluxcd.io/en/latest/)

## What Flux does

Flux is most useful when used as a deployment tool at the end of a
Continuous Delivery pipeline. Flux will make sure that your new
container images and config changes are propagated to the cluster.

### Who is using Flux in production

<table><tr valign="top">
<td>

 - [ABA English](https://www.abaenglish.com)
 - [Alea Play Solutions](http://www.alea.com)
 - [Apester](https://apester.com)
 - [APPUiO](https://appuio.ch)
 - [ArangoDB Oasis](https://arangodb.com/managed-service)
 - [Avisi](https://avisi.nl)
 - [B3i](https://b3i.tech/)
 - [Babylon Health](https://www.babylonhealth.com/)
 - [Belvo](https://belvo.com/)
 - [bimspot](https://bimspot.io)
 - [Bink](https://bink.com)
 - [BlaBlaCar](https://www.blablacar.com/)
 - [blackmirror media](https://blackmirror.media)
 - [Canva](https://www.canva.com/)
 - [Carrot](https://carrot.com)
 - [Chattigo](https://chattigo.com)
 - [Cloudlets](https://cloudlets.io)
 - [Cloudogu GmbH](https://cloudogu.com)
 - [CLVR Cloud Ltd](https://clvr.cloud)
 - [College of William & Mary](https://www.wm.edu)
 - [Consorcio Chile](https://www.consorcio.cl)
 - [Control Plane](https://control-plane.io)
 - [CrowdRiff](https://crowdriff.com)
 - [Cybrary](https://cybrary.it)
 - [Duedil](https://www.duedil.com/)
 - [Enliven Systems](https://enliven.systems)
 - [FELFEL](https://felfel.ch/)

</td><td>

 - [FireHydrant](https://firehydrant.io)
 - [Frame.io](https://frame.io)
 - [gastromatic](https://www.gastromatic.de/)
 - [Gini](https://gini.net)
 - [Greenpass](https://www.greenpass.com.br/)
 - [happn](https://www.happn.com)
 - [HMCTS](https://www.gov.uk/government/organisations/hm-courts-and-tribunals-service)
 - [IBM Cloudant](https://www.ibm.com/cloud/cloudant)
 - [Improwised Technologies](https://www.improwised.com/)
 - [Infabode](https://infabode.com)
 - [iQmetrix](https://www.iqmetrix.com)
 - [karriere tutor GmbH](https://www.karrieretutor.de)
 - [Limejump](https://limejump.com)
 - [loveholidays](https://www.loveholidays.com/)
 - [Luma Health](https://lumahealth.io/)
 - [LUNAR](https://www.lunarway.com/)
 - [MediaMarktSaturn](https://www.mediamarktsaturn.com)
 - [Mettle](https://mettle.co.uk)
 - [Mintel](https://www.mintel.com)
 - [Multicaja S.A](https://www.multicaja.cl)
 - [MyFitnessPal](https://www.myfitnesspal.com)
 - [Okteto Cloud](https://okteto.com/)
 - [Omise](https://www.omise.co)
 - [Payout](https://payout.one)
 - [Plex Systems](https://www.plex.com/)
 - [Qordoba](https://qordoba.com)
 - [Quivr](https://quivr.be)

</td><td>

 - [Rakuten](https://rakuten.com)
 - [RentPath](https://rentpath.com)
 - [Replicated](https://replicated.com)
 - [Resulta](https://resulta.com)
 - [Rungway](https://rungway.com)
 - [Sage AI Labs](https://www.sage.com)
 - [Sngular](https://www.sngular.com)
 - [Starbucks](https://www.starbucks.com/)
 - [SupplyStack](https://www.supplystack.com/)
 - [Swoop](https://www.swoopfunding.com)
 - [Talend](https://www.talend.com)
 - [Troii](https://troii.com/)
 - [UK Hydrographic Office](https://www.gov.uk/government/organisations/uk-hydrographic-office)
 - [Under Armour](https://www.underarmour.com)
 - [VSHN](https://vshn.ch)
 - [Walmart Chile](https://www.walmartchile.cl)
 - [Weave Cloud](https://cloud.weave.works)
 - [Wildlife Studios](https://wildlifestudios.com)
 - [Workable](https://www.workable.com)
 - [Workarea](https://www.workarea.com)
 - [Working Group Two](https://wgtwo.com)
 - [Yad2](https://yad2.co.il)
 - [Yusofleet](https://yusofleet.com)
 - [Zaaksysteem](https://zaaksysteem.nl)
 - [Zwift](https://zwift.com)

</td></tr></table>

If you too are using Flux in production; please submit a PR to add your organization to the list!

### History

In the first years of its existence, the development of Flux was very
closely coupled to that of [Weave
Cloud](https://www.weave.works/product/cloud/). Over the years the community
around Flux grew, the numbers of [integrations](#integrations) grew and
the team started the process of generalising the code, so that more projects
could easily integrate.

## Get started with Flux

With the following tutorials:

- [Get started with Flux](https://docs.fluxcd.io/en/latest/tutorials/get-started)
- [Get started with Flux using Helm](https://docs.fluxcd.io/en/latest/tutorials/get-started-helm)

or just [browse through the documentation](https://docs.fluxcd.io).

Do you want to release your Helm charts in a declarative way?
Take a look at the [`fluxcd/helm-operator`](https://github.com/fluxcd/helm-operator).

### Integrations

As Flux is Open Source, integrations are very straight-forward. Here are
a few popular ones you might want to check out:

- [Manage a multi-tenant cluster with Flux and Kustomize](https://github.com/fluxcd/multi-tenancy)
- [Managing Helm releases the GitOps way](https://github.com/fluxcd/helm-operator-get-started)
- [OpenFaaS GitOps workflow with Flux](https://github.com/stefanprodan/openfaas-flux)
- [GitOps for Istio Canary deployments](https://github.com/stefanprodan/gitops-istio)
- [Fluxcloud to receive events from Flux](https://github.com/topfreegames/fluxcloud)

## Community & Developer information

We welcome all kinds of contributions to Flux, be it code, issues you found,
documentation, external tools, help and support or anything else really.

The Flux project adheres to the [CNCF Code of
Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

Instances of abusive, harassing, or otherwise unacceptable behavior
may be reported by contacting a _Flux_ project maintainer, or the CNCF
mediator, Mishi Choudhary <mishi@linux.com>.

To familiarise yourself with the project and how things work, you might
be interested in the following:

- [Our contributions guidelines](CONTRIBUTING.md)
- [Build documentation](https://docs.fluxcd.io/en/latest/contributing/building)
- [Release documentation](internal/docs/releasing.md)

## <a name="help"></a>Getting Help

Reminder that Flux v1 is in maintenance mode. If you have any questions about Flux v2 and future migrations, these are the best ways to stay informed:
- Read about the [GitOps Toolkit](https://toolkit.fluxcd.io/) (Flux v2 is built on the GitOps Toolkit).
- Ask questions and add suggestions in our [GitOps Toolkit Discussions](https://github.com/fluxcd/toolkit/discussions)

- Watch our [featured talks on updates to Flux v2 and the GitOps Toolkit](https://github.com/fluxcd/toolkit#featured-talks)
- Join the next talk and Q&A about Flux v2 on [Dec 14, 10:00 am PT / 18:00 GMT](https://www.meetup.com/GitOps-Community/events/274924513/)

- Join the [flux-dev mailing list](https://lists.cncf.io/g/cncf-flux-dev).
- Join the Flux v2 / GitOps Toolkit [community meetings](https://github.com/fluxcd/flux2#community)

If you have further questions about Flux or continuous delivery:

- Read [the Flux docs](https://docs.fluxcd.io).
- Invite yourself to the <a href="https://slack.cncf.io" target="_blank">CNCF community</a>
  slack and ask a question on the [#flux](https://cloud-native.slack.com/messages/flux/)
  channel.
- To be part of the conversation about Flux's development, join the
  [flux-dev mailing list](https://lists.cncf.io/g/cncf-flux-dev).
- [File an issue.](https://github.com/fluxcd/flux/issues/new/choose)

Your feedback is always welcome!
