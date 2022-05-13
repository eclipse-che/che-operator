# Eclipse Che Helm Charts

- [Charts](#charts)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)


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
  --set ingress.domain=<KUBERNETES_INGRESS_DOMAIN> \
  --set ingress.auth.oAuthSecret=<OAUTH_SECRET> \
  --set ingress.auth.oAuthClientName=<OAUTH_CLIENT_NAME> \
  --set ingress.auth.identityProviderURL=<IDENTITY_PROVIDER_URL> .
```
