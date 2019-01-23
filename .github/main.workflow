workflow "Publish Helm chart" {
  on = "push"
  resolves = ["helm-gh-pages"]
}

action "helm-gh-pages" {
  uses = "stefanprodan/gh-actions/helm-gh-pages@master"
  args = ["chart/flux","https://weaveworks.github.io/flux","chart-"]
  secrets = ["GITHUB_TOKEN"]
}
