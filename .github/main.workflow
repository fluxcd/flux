workflow "Publish Helm chart" {
  on = "push"
  resolves = ["helm-gh-pages"]
}

action "helm-lint" {
  uses = "stefanprodan/gh-actions/helm@master"
  args = ["lint chart/flux"]
}

action "tag-filter" {
  needs = ["helm-lint"]
  uses = "actions/bin/filter@master"
  args = "tag chart-*"
}

action "helm-gh-pages" {
  needs = ["tag-filter"]
  uses = "stefanprodan/gh-actions/helm-gh-pages@master"
  args = ["chart/flux","https://weaveworks.github.io/flux"]
  secrets = ["GITHUB_TOKEN"]
}
