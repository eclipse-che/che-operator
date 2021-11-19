# Eclipse Che Helm Charts

- [Charts](#charts)
  - [Kubernetes Versions](#kubernetes-versions)
  - [Helm Versions](#helm-versions)
  - [Helm Chart Install](#helm-chart-install)


## Charts

Helm charts to deploy [Eclipse Che](https://www.eclipse.org/che/)

### Kubernetes Versions

Minimal Kubernetes version is 1.19

### Helm Versions

Helm charts are only tested with Helm version 3.7.1

### Helm Chart Install

Create a Namespace and install the helm chart for Eclipse Che.

```
NAMESPACE=eclipse-che
DOMAIN=<KUBERNETES_CLUSTER_DOMAIN>

kubectl create namespace $NAMESPACE

# Install charts
helm install che --set k8s.ingressDomain=$DOMAIN --namespace $NAMESPACE .
```
