# applicationset-previewer

Takes ArgoCD ApplicationSets definitions and (optionally) cluster secrets to
generate the resulting Applications.

Usage:

```sh
go run ./hack/applicationset-previewer [list-of-yamls]
```

Cluster secrets can be fetched from your ArgoCD instance locally through:

```sh
kubectl get secret -l argocd.argoproj.io/secret-type=cluster -n argocd -o yaml |\
    yq e '.items[] | splitDoc | del(.data.config) | del(.metadata.annotations."kubectl.kubernetes.io/last-applied-configuration")' -
```

Note the `yq` invocation to remove sensitive information (like the connection
credentials) from the secrets and splitting the List into separate YAML
documents. We only need `metadata.labels`, `data.name` and `data.server` for
correct processing.

This currently supports the following generators:

* `list`
* `clusters`
* `matrix`
* `merge`

Not yet supported:

* `git`
* `scmProvider`
* `clusterDecisionResource`
* `pullRequest`
* `plugin`
