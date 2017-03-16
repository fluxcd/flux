## 0.2.0 (2017-03-16)

More informative and helpful UI.

### Features

-   Lots more documentation
-   More informative output from `fluxctl release`
-   Added option in `fluxctl set-config` to generate a deploy key

### Improvements

-   Slack notifications are tidier
-   Support for releasing to >1 service at a time
-   Better behaviour when flux deploys itself
-   More help given for commonly encountered errors
-   Filter out Kubernetes add-ons from consideration
-   More consistent Prometheus metric labeling

See also https://github.com/weaveworks/flux/issues?&q=closed%3A"2017-01-27 .. 2017-03-15"

## 0.1.0 (2017-01-27)

Initial semver release.

### Features

-   Validate image release requests.
-   Added version command

### Improvements

-   Added rate limiting to prevent registry 500's
-   Added new release process
-   Refactored registry code and improved coverage

See https://github.com/weaveworks/flux/milestone/7?closed=1 for full details.

