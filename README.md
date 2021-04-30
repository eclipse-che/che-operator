# Che/CodeReady Workspaces Operator

[![codecov](https://codecov.io/gh/eclipse-che/che-operator/branch/main/graph/badge.svg?token=IlYvrVU5nB)](https://codecov.io/gh/eclipse-che/che-operator)

Che/CodeReady workspaces operator uses [Operator SDK](https://github.com/operator-framework/operator-sdk) and [Go Kube client](https://github.com/kubernetes/client-go) to deploy, update and manage K8S/OpenShift resources that constitute a single or multi-user Eclipse Che/CodeReady Workspaces cluster.

The operator watches for a Custom Resource of Kind `CheCluster`, and operator controller executes its business logic when a new Che object is created, namely:

* creates k8s/OpenShift objects
* verifies successful deployment of Postgres, Keycloak, Devfile/Plugin registries and Che server
* runs exec into Postgres and Keycloak pods to provisions databases, users, realm and clients
* updates CR spec and status (passwords, URLs, provisioning statuses etc.)
* continuously watches CR, update Che ConfigMap accordingly and schedule a new Che deployment
* changes state of certain objects depending on CR fields:
    * turn on/off TLS mode (reconfigure routes, update ConfigMap)
    * turn on/off OpenShift oAuth (login with OpenShift in Che) (create identity provider, oAuth client, update Che ConfigMap)
* etc

Che operator is implemented using [operator framework](https://github.com/operator-framework) and the Go programming language. Eclipse Che configuration defined using a custom resource definition object and stored in the custom Kubernetes resource named CheCluster (Kubernetes API group `org.eclipse.checluster`). Che operator extends Kubernetes API to embed Eclipse Che to Kubernetes cluster in a native way.

## CheCluster custom resource

Che operator deploys Eclipse Che using configuration stored in the Kubernetes custom resource(CR). CR object structure defined in the code using `pkg/apis/org/v1/che_types.go` file. Field name defined using the serialization tag `json`, for example `json:"openShiftoAuth"`. Che operator default CR sample is stored in the `deploy/crds/org_v1_che_cr.yaml`. This file should be directly modified if you want to apply new fields with default values, or in case of changing default values for existing fields.
Also, you can apply in the field comments Openshift UI annotations: to display some
interactive information about these fields on the Openshift UI.
For example:

```go
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Eclipse Che URL"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:org.w3:link"
```

This comment-annotations displays clickable link on the Openshift ui with a text "Eclipse Che URL"

It is mandatory to update the OLM bundle after modification of the CR sample to deploy Eclipse Che using OLM.

## Build and push custom Che operator image

1. Export environment variables:

```bash
$ export IMAGE_REGISTRY_USER_NAME=<IMAGE_REGISTRY_USER_NAME> && \
  export IMAGE_REGISTRY_HOST=<IMAGE_REGISTRY_HOST> && \
```

Where:
- `IMAGE_REGISTRY_USER_NAME` - docker image registry account name.
- `IMAGE_REGISTRY_HOST` - docker image registry hostname, for example: "docker.io", "quay.io". Host could be with a non default port: localhost:5000, 127.0.0.1:3000 and etc.

2. Run VSCode task `Build and push custom che-operator image` or use the terminal:

```bash
$ docker build -t ${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly .
$ docker push ${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly
```

## Deploy Che operator

### Deploy Che operator with chectl

To deploy Che operator you can use [chectl](https://github.com/che-incubator/chectl). It has got two installer types corresponding to Che operator: `operator` and `olm`. With the `--installer operator` chectl reuses copies of Che operator deployment and roles (cluster roles) YAMLs, CR, CRD from the `deploy` directory of the project. With `--installer olm` chectl uses catalog source index image with olm bundles from the `deploy/olm-catalog` directory.

#### Deploy Che operator with chectl using `--installer operator` flag

1. Build your custom operator image, see [How to Build che-operator Image](#build-and-push-custom-che-operator-image).

2. Deploy Eclipse Che on a running k8s cluster:

```bash
$ chectl server:deploy --installer operator -p <PLATFORM> --che-operator-image=${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly
```

Where:
- `PLATFORM` - k8s platform supported by chectl.

> INFO: if you have changed Che operator deployment, roles, cluster roles, CRD or CR then you must use `--templates` flag to point chectl to modified Che operator templates. Copy all files from the `deploy` folder of the che-operator project into a folder `<SOME_PATH>/templates/che-operator` and use it with chectl:

```bash
$ chectl server:deploy --installer operator -p <PLATFORM> --che-operator-image=${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly --templates <SOME_PATH>/templates
```

#### Deploy Che operator with chectl using `--installer olm` flag

1. Build your custom operator image, see [How to Build Che operator Image](#build-and-push-custom-che-operator-image).

2. Create newer OLM files:

```bash
$ olm/update-resources.sh
```

3. Build catalog source and bundle images:

```bash
$ olm/buildAndPushBundleImages.sh -p <openshift|kubernetes> -c "nightly"
```

4. Create a custom catalog source yaml (update strategy is workaround for https://github.com/operator-framework/operator-lifecycle-manager/issues/903):

```yaml
apiVersion:  operators.coreos.com/v1alpha1
kind:         CatalogSource
metadata:
  name:         eclipse-che-preview-custom
  namespace:    che-namespace
spec:
  image:        <IMAGE_REGISTRY_HOST>/<IMAGE_REGISTRY_USER_NAME>/eclipse-che-<openshift|kubernetes>-opm-catalog:preview
  sourceType:  grpc
  updateStrategy:
    registryPoll:
      interval: 5m
```

5. Deploy Che operator:

```bash
$ chectl server:deploy --installer=olm --platform=<CHECTL_SUPPORTED_PLATFORM> --catalog-source-yaml <PATH_TO_CUSTOM_CATALOG_SOURCE_YAML> --olm-channel=nightly --package-manifest-name=eclipse-che-preview-<openshift|kubernetes>
```

### Deploy Che operator using bash script

> WARNING: Cluster Admin privileges are required

```bash
./deploy.sh $namespace
```

The script creates service account, roles, roles binding, operator deployment, CRD, and CR resources. Wait until Che deployment is scaled to 1 and Che pod is run. Make sure you provide a global ingress domain in `deploy/crds/org_v1_che_cr.yaml` for k8s platform, for example:

```bash
  k8s:
    ingressDomain: '192.168.99.101.nip.io'
```

## Deploy Che operator for different usecases

### Single user mode

Che operator deploys Eclipse Che with enabled multi-user mode by default. To start work each user should login/register using form, after that user will be redirected to the user dashboard.

To enable single user mode use the command line:

```bash
$ kubectl patch checluster/eclipse-che -n <ECLIPSE-CHE-NAMESPACE> --type=merge -p '{"spec":{"server": {"customCheProperties": {"CHE_MULTIUSER": "false"}}}}'
```

or create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  server:
    customCheProperties:
      CHE_MULTIUSER: "false"
```

```bash
$ chectl server:update -n <ECLIPSE-CHE-NAMESPACE> --che-operator-cr-patch-yaml <PATH_TO_CR_PATCH_YAML>
```

### Workspace namespace strategy

Workspace namespace strategy defines default namespace in which user's workspaces are created.
It's possible to use <username>, <userid> and <workspaceid> placeholders (e.g.: che-workspace-<username>).
In that case, new namespace will be created for each user (or workspace).
For OpenShift infrastructure this property used to specify Project (instead of namespace conception).

To set up namespace workspace strategy use command line:

```bash
$ kubectl patch checluster/eclipse-che -n <ECLIPSE-CHE-NAMESPACE> --type=merge -p '{"spec":{"server": {"customCheProperties": {"CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT": "che-workspace-<username>"}}}}'
```

or create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  server:
    customCheProperties:
      CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT: "che-workspace-<username>"
```

```bash
$ chectl server:update -n <ECLIPSE-CHE-NAMESPACE> --che-operator-cr-patch-yaml <PATH_TO_CR_PATCH_YAML>

### OpenShift OAuth

OpenShift clusters include a built-in OAuth server. Che operator supports this authentication method. It's enabled by default.

To disable OpenShift OAuth use command line:

```bash
$ kubectl patch checluster/eclipse-che -n <ECLIPSE-CHE-NAMESPACE> --type=merge -p '{"spec":{"auth":{"openShiftoAuth": false}}}'
```

or create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  auth:
    openShiftoAuth: false
```

```bash
$ chectl server:update -n <ECLIPSE-CHE-NAMESPACE> --che-operator-cr-patch-yaml <PATH_TO_CR_PATCH_YAML>
```

### TLS

TLS is enabled by default. Turning it off is not recommended as it will cause malfunction of some components. But for development purposes you can do that:

```bash
$ kubectl patch checluster/eclipse-che -n <ECLIPSE-CHE-NAMESPACE> --type=merge -p '{"spec":{"server":{"tlsSupport": false}}}'
```

or create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  server:
    tlsSupport: false
```

```bash
$ chectl server:update -n <ECLIPSE-CHE-NAMESPACE> --che-operator-cr-patch-yaml <PATH_TO_CR_PATCH_YAML>
```

#### TLS with OpenShift

When the cluster is configured to use self-signed certificates for the router, the certificate is automatically propagated to Che components as trusted. If cluster router uses certificate signed by self-signed one, then parent/root CA certificate should be added into corresponding config map of additional trusted certificates (see `serverTrustStoreConfigMapName` option).

#### TLS with K8S

By default self-signed certificates for Che will be generated automatically. If it is needed to use your own certificates, create `che-tls` secret (see `k8s.tlsSecretName` option) with `key.crt` and `tls.crt` fields. In case of self-signed certificate `self-signed-certificate` secret should be created with the public part of CA certificate under `ca.crt` key in secret data. It is possible to use default certificate of Kubernetes cluster by passing empty string as a value of tlsSecretName:

```yaml
spec:
  k8s:
    tlsSecretName: ''
```

## Update Che operator deployment

### Edit checluster custom resource using a command-line interface (terminal)

You can modify any Kubernetes object using the UI (for example OpenShift web console) or you can also modify Kubernetes objects using the terminal:

```bash
$ kubectl edit checluster eclipse-che -n <ECLIPSE-CHE-NAMESPACE>
```

or:

```bash
$ kubectl patch checluster/eclipse-che --type=merge -p '<PATCH_JSON>' -n <ECLIPSE-CHE-NAMESPACE>
```

### Update checluster using chectl

You can update Che configuration using the `chectl server:update` command providing `--cr-patch` flag. See [chectl](https://github.com/che-incubator/chectl) for more details.

## Development

### Debug Che operator

You can run/debug this operator on your local machine (without deploying to a k8s cluster).

Go client grabs kubeconfig either from InClusterConfig or `~/.kube` locally. Make sure  your current kubectl context points to a target cluster and namespace and a current user can create objects in a target namespace.

```bash
`./local-debug.sh -n <ECLIPSE-CHE-NAMESPACE> -cr <CUSTOM_RESOURCE>
```

Where:
* `ECLIPSE-CHE-NAMESPACE` - namespace name to deploy Che operator into, default is `che`
* `CUSTOM_RESOURCE` - path to custom resource yaml, default is `./deploy/crds/org_v1_che_cr.yaml`

Use VSCode debug configuration `Che Operator` to attach to the running process.

### Run and debug mock tests

Che operator covered with mock tests. To run them use VSCode task `Run che-operator mock tests` or run in the terminal in the root of the project:

```bash
$ export MOCK_API="true"; go test -mod=vendor -v ./...
```

To debug Che operator tests you can use VSCode `Launch Current File` debug configuration.
For that you have to open file with a test, for example `pkg/controller/che/che_controller_test.go`, set up some breakpoints, select debug tab, select `Launch Current File` configuration in the debug panel and click the `Start debugging` button. Test will be executed with the environment variable `MOCK_API=true` to enable "mocks" mode.

### Run E2E tests

`e2e` directory contains end-to-end tests that create a custom resource, operator deployment, required RBAC.

Pre-reqs to run end-to-end (e2e) tests:

* a running Minishift cluster
* current oc/kubectl context as a cluster admin user

```bash
$ e2e/run_tests.sh
```

### Compile Che operator code

The operator will be compiled to the binary `/tmp/che-operator/che-operator`.
This command is useful to make sure that che-operator is still compiling after your changes. Run VSCode task: `Compile che-operator code` or use the terminal:

```bash
GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go
```

### Format code

Run the VSCode task: `Format che-operator code` or use the terminal:

```bash
$ go fmt ./...
```
> Notice: if you don't have redhat subscription, use public image registry.access.redhat.com/devtools/go-toolset-rhel7:latest

### Update golang dependencies

Che operator uses Go modules and a vendor folder. Run the VSCode task: `Update che-operator dependencies` or use the terminal:

```bash
$ go mod vendor
```

New golang dependencies in the vendor folder should be committed and included in the pull request.

### Updating Custom Resource Definition file

Che cluster custom resource definition (CRD) defines Eclipse CheCluster custom resource object. It contains information about object structure, field types, field descriptions. CRD file is a YAML definition located in the folder `deploy/crds`. These files are auto-generated, so do not edit it directly to update them. If you want to add new fields or fix descriptions in the CRDs, make your changes in the file `pkg/apis/org/v1/che_types.go` and run VSCode task `Update resources` or use the terminal

```bash
$ olm/update-resources.sh
```

> Notice: this script contains commands to make the CRD compatible with Openshift 3.

### Update nightly OLM bundle

Sometimes, during development, you need to modify some YAML definitions in the `deploy` folder or Che cluster custom resource. There are most frequently changes which should be included to the new OLM bundle:
  - operator deployment `deploy/operator.yaml`
  - operator roles/cluster roles permissions. They are defined like role/rolebinding or cluster role/rolebinding yamls in the `deploy` folder.
  - operator custom resource CR `deploy/crds/org_v1_che_cr.yaml`. This file contains the default CheCluster sample. Also this file is the default OLM CheCluster sample.
  - Che cluster custom resource definition `pkg/apis/org/v1/che_types.go`. For example you want to fix some properties description or apply new Che type properties with default values. These changes affect CRD `deploy/crds/org_v1_che_crd.yaml`.
  - add Openshift ui annotations for Che types properties (`pkg/apis/org/v1/che_types.go`) to display information or interactive elements on the Openshift user interface.

For all these cases it's a necessary to generate a new OLM bundle to make these changes working with OLM. Run the VSCode tasks `Update resources` or use the terminal:

```bash
$ olm/update-resources.sh
```

Every changes will be included to the `deploy/olm-catalog` bundles and will override all previous changes. OLM bundle changes should be committed to the pull request.

To update a bundle without version incrementation and time update you can use env variables `NO_DATE_UPDATE` and `NO_INCREMENT`. For example, during development you need to update bundle a lot of times with changed che-operator deployment or role, rolebinding and etc, but you don't want to increment the bundle version and time creation, when all desired changes were completed:

```bash
$ export NO_DATE_UPDATE="true" \
&& export NO_INCREMENT="true" \
&& olm/update-resources.sh
```

### Che operator PR checks

Documentation about all Che operator test cases can be found [here](https://github.com/eclipse-che/che-operator/tree/main/.ci/README.md)

### Generate go mocks.

Install mockgen tool:

```bash
$ GO111MODULE=on go get github.com/golang/mock/mockgen@v1.4.4
```

Generate new mock for go interface. Example:

```bash
$ mockgen -source=pkg/util/process.go -destination=mocks/pkg/util/process_mock.go -package mock_util

$ mockgen -source=pkg/controller/che/oauth_initial_htpasswd_provider.go \
          -destination=mocks/pkg/controller/che/oauth_initial_htpasswd_provider_mock.go \
          -package mock_che
```

See more: https://github.com/golang/mock/blob/master/README.md
