workflow "Publish Helm chart" {
  on = "push"
  resolves = ["helm-gh-pages"]
}

action "helm-lint" {
  uses = "stefanprodan/gh-actions/helm@master"
  args = ["lint chart/flux"]
}

action "helm-gh-pages" {
  needs = ["helm-lint"]
  uses = "stefanprodan/gh-actions/helm-gh-pages@master"
  args = ["chart/flux","https://weaveworks.github.io/flux","chart-"]
  secrets = ["GITHUB_TOKEN"]
}
