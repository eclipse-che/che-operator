# Eclipse Che Helm Charts

- [Charts](#charts)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)


## Charts

Helm charts to deploy [Eclipse Che](https://www.eclipse.org/che/)

### Prerequisites

* Minimal Kubernetes version is 1.19
* Minimal Helm version is 3.2.2

### Installation the next Helm chart

Create a Namespace and install the Helm Charts for Eclipse Che Operator.

```
NAMESPACE=eclipse-che
DOMAIN=<KUBERNETES_CLUSTER_DOMAIN>

kubectl create namespace $NAMESPACE

cd next

# Install charts
helm install che --set k8s.ingressDomain=$DOMAIN --namespace $NAMESPACE .
```

### Installation stable Helm charts

Use https://artifacthub.io to install the latest stable Helm charts.
