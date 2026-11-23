# Eclipse Che Helm Charts

- [Charts](#charts)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Upgrade](#upgrade)


## Charts

Helm charts to deploy [Eclipse Che](https://www.eclipse.org/che/)

### Prerequisites

* Minimal Kubernetes version is 1.19
* Minimal [Helm](https://helm.sh/) version is 3.2.2
* [Cert manager](https://cert-manager.io/docs/installation/) installed
* [OIDC Identity Provider](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#configuring-the-api-server) configured

### Installation
Install the Helm Charts for Eclipse Che Operator

```
helm install che \
  --create-namespace \ 
  --namespace eclipse-che \
  --set networking.domain=<KUBERNETES_INGRESS_DOMAIN> \
  --set networking.auth.oAuthSecret=<OAUTH_SECRET> \
  --set networking.auth.oAuthClientName=<OAUTH_CLIENT_NAME> \
  --set networking.auth.identityProviderURL=<IDENTITY_PROVIDER_URL> .
```

### Upgrade

Helm installs the CRDs from the `crds/` directory only on the first installation
and never upgrades them (see [Helm limitations on CRDs](https://helm.sh/docs/topics/charts/#limitations-on-crds)).
Before running `helm upgrade`, apply the CRDs from the new chart version manually.
If the chart is pulled from a remote repository, download it first with `helm pull`
and apply the CRDs from the unpacked chart directory:

```sh
helm pull <CHART> --untar
kubectl apply --server-side --force-conflicts -f <UNTARRED_CHART_DIR>/crds/
```

`--server-side` is required because the CRD is too large for a client-side apply
(the `kubectl.kubernetes.io/last-applied-configuration` annotation exceeds the size limit).
`--force-conflicts` lets `kubectl` take ownership of the fields that Helm set when it created the CRDs.

Then upgrade the release. Replace `<NAMESPACE>` with the namespace where Eclipse Che
is installed (`eclipse-che` by default). To find the name of the existing release, run
`helm list --namespace <NAMESPACE>`:

```sh
helm list --namespace <NAMESPACE>
helm upgrade <RELEASE_NAME> --namespace <NAMESPACE> <CHART>
```
