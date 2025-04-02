# Eclipse Che Helm Charts

- [Charts](#charts)
  - [About](#about)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)

## Charts

Helm charts to deploy [Eclipse Che](https://www.eclipse.org/che/)

### About

Eclipse Che is a cloud-based, Kubernetes-native development environment designed to
simplify and enhance the developer experience. It provides fully containerized workspaces,
allowing developers to write, build, and debug applications in a consistent
and reproducible manner. Eclipse Che eliminates the need for complex local setups
by offering pre-configured development environments that run in the cloud,
making collaboration easier across teams.

#### Key Features

* Cloud-Native Workspaces
* Integrated Development Environment (IDE)
* Kubernetes and OpenShift Support
* Preconfigured Stacks
* Team Collaboration
* Secure and Scalable

Eclipse Che simplifies onboarding, development, and collaboration 
by bringing full-fledged development environments to the cloud. 

For a detailed introduction to Eclipse Che, refer to the official documentation:  
https://eclipse.dev/che/docs/stable/overview/introduction-to-eclipse-che/

### Prerequisites

* Minimal Kubernetes version is 1.19
* Minimal [Helm](https://helm.sh/) version is 3.2.2

### Installation

#### Install [Cert Manager](https://cert-manager.io/docs/installation/helm/) using `Helm`

```sh
CERT_MANAGER_VERSION=v1.17.0

helm repo add jetstack https://charts.jetstack.io --force-update
helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version ${CERT_MANAGER_VERSION} \
  --set crds.enabled=true
```

#### Install [DevWorkspace operator](https://github.com/devfile/devworkspace-operator) using `kubectl`

1. Create `devworkspace-controller` namespace:
```sh
kubectl create namespace devworkspace-controller
```

2. Install the DevWorkspace Operator.
Check the [DevWorkspace Operator releases page](https://github.com/devfile/devworkspace-operator/tags) to find the latest version.
```sh
DEVWORKSPACE_OPERATOR_VERSION=v0.33.0
kubectl apply -f https://github.com/devfile/devworkspace-operator/raw/refs/tags/"${DEVWORKSPACE_OPERATOR_VERSION}"/deploy/deployment/kubernetes/combined.yaml 
```


3. Wait until the DevWorkspace Operator pods are ready:
```sh
kubectl wait --namespace devworkspace-controller \
      --timeout 90s \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/part-of=devworkspace-operator
```

3. Create the `DevWorkspaceOperatorConfig`, replacing `<CLUSTER_HOST_SUFFIX>` 
with the public domain name of the Kubernetes cluster.
```sh
kubectl apply -f - <<EOF
apiVersion: controller.devfile.io/v1alpha1
kind: DevWorkspaceOperatorConfig
metadata:
  name: devworkspace-operator-config
  namespace: devworkspace-controller
config:
  routing:
    clusterHostSuffix: "<CLUSTER_HOST_SUFFIX>"
EOF
```

#### Configure [OIDC Identity Provider](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#configuring-the-api-server) on the cluster.

Note that every Kubernetes cluster has its own specific configuration requirements 
for setting up an OIDC provider, such as unique issuer URLs, client IDs, and redirect URIs. 

#### Install [Eclipse Che Operator](https://github.com/eclipse-che/che-operator) using Helm

1. Create `ecipse-che` namespace
```sh
kubectl create namespace eclipse-che
```
2. Install the Eclipse Che Operator by clicking the `Install` button 
in the top-right corner and follow the instructions. Wait until the Operator is ready:
```sh
kubectl wait \
      --namespace eclipse-che \
      --timeout 90s \
      --for=condition=ready pod \
      --selector=app.kubernetes.io/component=che-operator
```

3. Click the `CRDs` button, select the `CheCluster` template, and then click the `Download` button.
Update the downloaded file `eclipse-che-CheCluster.yaml` by setting the following fields:
```yaml
spec:
  networking:
    domain: <...>
    auth:
      identityProviderURL: <...>
      oAuthClientName: <...>
      oAuthSecret: <...>
```

For more information on how to configure the `CheCluster`, see the [CheCluster Custom Resource fields reference](https://eclipse.dev/che/docs/stable/administration-guide/checluster-custom-resource-fields-reference/#checluster-custom-resource-networking-settings).

4. Create CheCluster CR:
```sh
kubectl apply -f eclipse-che-CheCluster.yaml -n eclipse-che
```

5. Wait until the Eclipse Che is ready:
```sh
kubectl wait checluster/eclipse-che \
      --namespace eclipse-che \
      --for=jsonpath='{.status.chePhase}'=Active \
      --timeout=360s
```

You can monitor the deployment process by viewing the Operator logs:
```sh
kubectl logs \
    --namespace eclipse-che \
    --selector app.kubernetes.io/component=che-operator \
    --follow
```

6. Open the Eclipse Che URL in a web browser:
```sh
kubectl get checluster/eclipse-che \
    --namespace eclipse-che \
    --output jsonpath="{.status.cheURL}"
```

Eclipse Che is ready to use!
