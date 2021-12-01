# Eclipse Che Helm Charts

A collaborative Kubernetes-native development solution that delivers Kubernetes workspaces and in-browser IDE for rapid cloud application development. This operator installs PostgreSQL, Keycloak, Plugin registry, Devfile registry and the Eclipse Che server, as well as configures all these services.

- [Charts](#charts)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)

## Charts

Helm charts to deploy [Eclipse Che](https://www.eclipse.org/che/)

### Prerequisites

* Minimal Kubernetes version is 1.19
* Minimal Helm version is 3.2.2

### Installation

Install `Eclipse Che Operator` by following instructions in top right button `Install`.

A new pod che-operator is created in `eclipse-che` namespace

```bash
$ kubectl get pods --all-namespaces | grep eclipse-che
eclipse-che   che-operator-554c564476-fl98z                           1/1     Running   0          13s
```

The operator is now providing new Custom Resources Definition: `checluster.org.eclipse.che`

Helm chart uses default Che Cluster custom resource to install Eclipse Che.
You can edit this custom resource to change configuration. 
***important:*** The operator is only tracking resources in its own namespace.

The operator will create pods for `Eclipse Che`. The deployment status can be tracked by looking at the Operator logs by using the command:

```bash
$ kubectl logs -n eclipse-che che-operator-554c564476-fl98z
important: pod name is different on each installation
```

When all Eclipse Che containers are running, the Eclipse Che URL is printed in the logs.

Eclipse Che URL can be tracked by searching for available trace:

```bash
$ kubectl logs -f -n eclipse-che che-operator-7b6b4bcb9c-m4m2m | grep "Eclipse Che is now available"
time="2019-08-01T13:31:05Z" level=info msg="Eclipse Che is now available at: http://che-eclipse-che.gcp.my-ide.cloud"
```

When Eclipse Che is ready, the Eclipse Che URL is displayed in CheCluster resource in status section

```bash
$ kubectl describe checluster/eclipse-che -n eclipse-che
```
```
Status:
Che Cluster Running:           Available
Che URL:                       http://che-eclipse-che.gcp.my-ide.cloud
Che Version:                   7.39.0
...
```

By opening this URL in a web browser, Eclipse Che is ready to use.

## Defaults
By default, the operator deploys Eclipse Che with:
* Bundled PostgreSQL and Keycloak
* Common PVC strategy
* Auto-generated passwords
* TLS mode (secure ingresses)
* Communicate between components using internal cluster SVC names

## Installation Options
Eclipse Che operator installation options include:
* Connection to external database and Keycloak
* Configuration of default passwords and object names
* PVC strategy (once shared PVC for all workspaces, PVC per workspace, or PVC per volume)
* Authentication options

Use `kubectl edit checluster/eclipse-che -n eclipse-che` to update Eclipse Che configuration.
See more configuration options in the [Installation guide](https://www.eclipse.org/che/docs/che-7/installation-guide/configuring-the-che-installation/).

### External Database and Keycloak
Follow the guides to configure external [Keycloak](https://www.eclipse.org/che/docs/che-7/administration-guide/configuring-authorization/#configuring-che-to-use-external-keycloak_che)
and [Database](https://www.eclipse.org/che/docs/che-7/administration-guide/external-database-setup/) setup.

### Certificates and TLS Secrets
Eclipse Che uses auto-generated self-signed certificates by default and TLS mode is on. To use a default certificate of a Kubernetes cluster set empty value in spec.k8s.tlsSecretName field:

```bash
kubectl patch checluster/eclipse-che --type=json -p '[{"op": "replace", "path": "/spec/k8s/tlsSecretName", "value": ""}]' -n eclipse-che
```