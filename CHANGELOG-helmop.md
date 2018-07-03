## 0.1.0-alpha (2018-05-01)

First versioned release of the Flux Helm operator. The target features are:

 - release Helm charts as specified in FluxHelmRelease resources
   - these refer to charts in a single git repo, readable by the operator
   - update releases when either the FluxHelmRelease resource or the
     chart (in git) changes

See
https://github.com/weaveworks/flux/blob/helm-0.1.0-alpha/site/helm/
for more detailed explanations.
