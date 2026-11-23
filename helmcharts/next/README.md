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
Before running `helm upgrade`, apply the CRDs from the new chart version manually:

```sh
kubectl apply --server-side --force-conflicts -f crds/
```

Then upgrade the release:

```sh
helm upgrade che --namespace eclipse-che <CHART>
```
