# Eclipse Che Helm Charts

- [Charts](#charts)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)


## Charts

Helm charts to deploy [Eclipse Che](https://www.eclipse.org/che/)

### Prerequisites

* Minimal Kubernetes version is 1.19
* Minimal Helm version is 3.2.2

### Installation

Install the Helm Charts for Eclipse Che Operator.

```
kubectl create namespace eclipse-che

DOMAIN=<KUBERNETES_CLUSTER_DOMAIN>

# Install charts
helm install che --set k8s.ingressDomain=$DOMAIN .
```
