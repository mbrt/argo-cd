# applicationset-previewer

Takes ArgoCD ApplicationSets definitions and (optionally) cluster secrets to
generate the resulting Applications.

Usage:

```sh
go run ./hack/applicationset-previewer [list-of-yamls]
```

Cluster secrets can be fetched from your ArgoCD instance locally through:

```sh
kubectl get -l argocd.argoproj.io/secret-type=cluster -n argocd secret -o yaml |\
    grep -v '^    config:'
```

Note the `grep` to remove sensitive information (like the connection
credentials) from the secrets. We only need `metadata.labels`, `data.name` and
`data.server`.

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
