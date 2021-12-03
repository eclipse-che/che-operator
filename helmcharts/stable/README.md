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

Create namespace `eclipse-che`:

```bash
$ kubectl create namespace eclipse-che
```

Install `Eclipse Che Operator` by following instructions in top right button `Install`.

A new pod che-operator is created in `eclipse-che` namespace

```bash
$ kubectl get pods -n eclipse-che
eclipse-che   che-operator-554c564476-fl98z                           1/1     Running   0          13s
```

Click `CRDS` button, select `CheCluster` template and copy 
custom resource Eclipse Che to file org.eclipse.che_v1_checluster.yaml.
Set up valid actual ingress domain `k8s.domain`.
Apply CR:

```bash
$ kubectl apply -f org.eclipse.che_v1_checluster.yaml -n eclipse-che
```

Also you can use `kubectl edit checluster/eclipse-che -n eclipse-che` to update Eclipse Che configuration.
See more configuration options in the [Installation guide](https://www.eclipse.org/che/docs/che-7/installation-guide/configuring-the-che-installation/).

The deployment process can be tracked by looking at the Operator logs by using the command:

```bash
$ kubectl logs che-operator-554c564476-fl98z -n eclipse-che -f
important: pod name is different on each installation
```

When all Eclipse Che containers are running, the Eclipse Che URL is printed in the logs:

```bash
time="2019-08-01T13:31:05Z" level=info msg="Eclipse Che is now available at: http://che-eclipse-che.gcp.my-ide.cloud"
```

By opening this URL in a web browser, Eclipse Che is ready to use.
