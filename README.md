## Che/CodeReady Workspaces Operator

Che/CodeReady workspaces operator uses [Operator SDK](https://github.com/operator-framework/operator-sdk) and [Go Kube client](https://github.com/kubernetes/client-go) to deploy, update and manage K8S/OpenShift resources that constitute a single or multi user Eclipse Che/CodeReady Workspaces cluster.

The operator watches for a Custom Resource of Kind `CheCluster`, and operator controller executes its business logic when a new Che object is created, namely:

* creates k8s/OpenShift objects
* verifies successful deployment of Postgres, Keycloak and Che
* runs exec into Postgres and Keycloak pods to provisions databases, users, realm and clients
* updates CR spec and status (passwords, URLs, provisioning statuses etc.)
* continuously watches CR, update Che ConfigMap accordingly and schedule a new Che deployment
* changes state of certain objects depending on CR fields:
    * turn on/off TLS mode (reconfigure routes, update ConfigMap)
    * turn on/off OpenShift oAuth (login with OpenShift in Che) (create identity provider, oAuth client, update Che ConfigMap)
* updates Che deployment with a new image:tag when a new operator version brings in a new Che tag

### Build custom che-operator image

You can build a custom che-operator image using command in the root of the che-operator project:

```bash
$ docker build -t ${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly .
```

> Notice: you can use VSCode task `Build and push custom che-operator image`. But you need to specify env variables: 
IMAGE_REGISTRY_HOST, IMAGE_REGISTRY_USER_NAME(for regular development you can use `.bashrc` file for this purpose).
This task will build and push image with name `${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly`
to the registry.

### Using chectl to deploy che-operator

To test che-operator changes you can use chectl: https://github.com/che-incubator/chectl
chectl has got two installer types corresponding to che-operator: `operator` and `olm`.
With `operator` installer chectl reuses copies of che-operator deployment and role(cluster role) yamls, CR, CRD from folder `deploy`.
With `olm` installer chectl uses catalog source index image with olm bundles based on `deploy/olm-catalog`.
chectl supports cluster platforms: "minikube", "minishift", "k8s", "openshift", "microk8s", "docker-desktop", "crc".

### Deploy che-operator with chectl via `operator` installer

If you want to test modified che-operator using chectl, you have to build a custom che-operator image.
See more: [Build custom che-operator image](#build-custom-che-operator-image)

Launch test cluster. Install Eclipse Che:

```bash
$ chectl deploy:server -n che --installer operator -p ${platform} --che-operator-image=${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly
```

where `platform` it's a cluster platform supported by chectl.

> INFO: if you changed che-operator deployment or role/cluster role, CRD, CR you have to provide `--templates` argument.
This argument will points chectl to your modificated che-operator `deploy` folder path.

### Deploy che-operator with chectl via OLM

The following instructions show how to test Che operator under development using OLM installer.

1. Build your custom operator image and use it in the operator deployment: [How to Build che-operator Image](#how-to-build-operator-image).
Push operator image to an image registry.

2. Create newer OLM files by executing: `olm/update-nightly-bundle.sh`

3. Build catalog source and bundle images.
Use `olm/buildAndPushInitialBundle.sh` script with `platform` argument('openshift' or 'kubernetes'):

```bash
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
  olm/buildAndPushInitialBundle.sh ${platform}
```

Where are:
  - `IMAGE_REGISTRY_USER_NAME` - your user account name in the image registry.
  - `IMAGE_REGISTRY_HOST` - host of the image registry, for example: "docker.io", "quay.io". Host could be with a non default port: localhost:5000, 127.0.0.1:3000 and etc.

4. Create custom catalog source yaml(update strategy is workaround for https://github.com/operator-framework/operator-lifecycle-manager/issues/903):

```yaml
apiVersion:  operators.coreos.com/v1alpha1
kind:         CatalogSource
metadata:
  name:         eclipse-che-preview-custom
  namespace:    che-namespace
spec:
  image:        ${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${PLATFORM}-opm-catalog:preview
  sourceType:  grpc
  updateStrategy:
    registryPoll:
      interval: 5m
```
Replace value of `image` field with your catalog source image. Don't forget to specify the desired platform.

5. Deploy Che using chectl:
```sh
$ chectl server:deploy --installer=olm --platform=${platform} -n ${che-namespace} --catalog-source-yaml ${path_to_custom_catalog_source_yaml} --olm-channel=nightly --package-manifest-name=eclipse-che-preview-${platform}
```

## Deploy che-operator using scripts

**IMPORTANT! Cluster Admin privileges are required**

```bash
./deploy.sh $namespace
```

The script will create sa, role, role binding, operator deployment, CRD and CR.

Wait until Che deployment is scaled to 1 and Che route is created.

When on pure k8s, make sure you provide a global ingress domain in `deploy/crds/org_v1_che_cr.yaml` for example:

```bash
  k8s:
    ingressDomain: '192.168.99.101.nip.io'
```

### Edit checluster CR using command line interface(terminal)

Any Kubernetes object you can modify using the UI(for example Openshift console).
But also you can do the same using terminal.
You can edit checluster CR object using command line editor: 

```bash
$ oc edit checluster ${che-cluster-name} -n ${namespace}
```

Where ${che-cluster-name} is a custom resource name, by default 'eclipse-che'.

Also you can modify the CheCluster using the `kubectl patch`. For example:

```bash
$ kubectl patch checluster/eclipse-che -n ${eclipse-che-namespace} --type=merge -p '{"spec":{"auth":{"openShiftoAuth": false}}}'
```

### Update checluster using chectl

You can update Che configuration using: command `chectl server:update` and flag `--cr-patch`

## OpenShift oAuth

OpenShift clusters includes a built-in OAuth server. che-operator supports this authentication way.
There is CR property 'openShiftoAuth' to enable/disable this feature. It's enabled by default.

To disable this feature:

- using command line:

```bash
$ kubectl patch checluster/eclipse-che -n ${eclipse-che-namespace} --type=merge -p '{"spec":{"auth":{"openShiftoAuth": false}}}'
```

- Also you can create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  auth:
    openShiftoAuth: false
```

And update che-cluster using chectl:

```
$ chectl server:update -n ${eclipse-che-namespace} --che-operator-cr-patch-yaml=/path/to/cr-patch.yaml
```

> INFO: If you are using scripts to deploy che-operator, then  bear in mind that che-operator service account needs to have cluster admin privileges so that the operator can create oauthclient at a cluster scope.
There is `oc adm` command in both deploy scripts. Uncomment it if you need this feature.
Make sure your current user has cluster-admin privileges.

## TLS

TLS is enabled by default.
Turning it off is not recommended as it will cause malfunction of some components.
But for development purposes you can do that.

- using command line:

```bash
$ kubectl patch checluster/eclipse-che -n ${eclipse-che-namespace} --type=merge -p '{"spec":{"server":{"tlsSupport": false}}}'
```

- Also you can create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  server:
    tlsSupport: false
```

And update che-cluster using chectl:

```
$ chectl server:update -n ${eclipse-che-namespace} --che-operator-cr-patch-yaml=/path/to/cr-patch.yaml
```

### Deploy multi-user Che

che-operator deploys Eclipse Che with enabled multi-user mode by default.
To start work each user should login/register using form, after that user will be redirected to the user dashboard.

### Deploy single user Che

To enable single user Che you can:

- use command line:

```bash
$ kubectl patch checluster/eclipse-che -n ${eclipse-che-namespace} --type=merge -p '{"spec":{"server": {"customCheProperties": {"CHE_MULTIUSER": "false"}}}}'
```

- Also you can create `cr-patch.yaml` and use it with chectl:

```yaml
spec:
  server:
    customCheProperties:
      CHE_MULTIUSER: "false"
```

And update che-cluster using chectl:

```
$ chectl server:update -n ${eclipse-che-namespace} --che-operator-cr-patch-yaml=/path/to/cr-patch.yaml
```

### Deploy Che with single host strategy

todo

#### OpenShift

When the cluster is configured to use self-signed certificates for the router, the certificate will be automatically propagated to Che components as trusted.
If cluster router uses certificate signed by self-signed one, then parent/root CA certificate should be added into corresponding config map of additional trusted certificates (see `serverTrustStoreConfigMapName` option).

#### K8S

By default self-signed certificates for Che will be generated automatically.
If it is needed to use your own certificates, create `che-tls` secret (see `k8s.tlsSecretName` option) with `key.crt` and `tls.crt` fields. In case of self-signed certificate `self-signed-certificate` secret should be created with the public part of CA certificate under `ca.crt` key in secret data.

## How to Configure

The operator watches all objects it creates and reconciles them with CR state. It means that if you edit a configMap **che**, the operator will revert changes.
Since not all Che configuration properties are custom resource spec fields (there are simply too many of them), the operator creates a second configMap called **custom**
which you can use for any environment variables not supported by the CR field. The operator will not reconcile configMap custom.

## Build and Deploy to a local cluster:

There's a little script that will build a local Docker image and deploy an operator to a selected namespace,
as well as create a service account, role, role binding, CRD and example CR.

```
oc new-project $namespace
build_deploy_local.sh $namespace

```

The above method will work only with Docker 17.x (does not work if you want to build in MiniShift/MiniKube). Mostly useful if you run `oc cluster up` locally.

### Local debug che-operator(outside cluster)

You can run/debug this operator on your local machine (not deployed to a k8s cluster)

Go client grabs kubeconfig either from InClusterConfig or `~/.kube` locally.
Make sure you oc login for Openshift cluster (or your current kubectl context points to a target cluster and namespace),
and a current user/server account can create objects in a target namespace.

To run che-operator in debug mode you can use `local-debug.sh` script. After process execution you should attach IDE debugger to debug port: 2345. Also you can see this port number in the process output.
`local-debug.sh` script has got two arguments:
 - `-n` - Eclipse Che installation namespace. Default value is `che`.
 - `-cr` - path to the custom resource yaml definition. Default value points to file `./deploy/crds/org_v1_che_cr.yaml`.

For VSCode we have launch.json configuration `Che Operator`. So in the VSCode debugger you can find this debug
configuration, select it, click `Start debugging` button. VSCode will attach a debugger to the che-operator process.

## Debug test scripts

che-operator has a lot of scripts and e2e test scripts to check che-operator code and detect regressions.
This scripts using shell.
To debug test scripts you can use the "Bash debug" VSCode extension.
For a lot of test scripts you can find different debug configurations in the `.vscode/launch.json`.

## Mock tests
che-operator covered with mock tests.

### Execute tests

You can execute tests using command:

```bash
$ export MOCK_API="true"; go test -mod=vendor -v ./...
```

Also you can use VSCode task `Run che-operator mock tests`.

### Debug che-operator mock tests in VSCode 

To debug che-operator tests you can use VSCode `Launch Current File` debug configuration.
For that you have to open file with test, for example `pkg/controller/che/che_controller_test.go`,
set up some breakpoints, select debug tab, select `Launch Current File` configuration in the debug panel
and click the `Start debugging` button. Test will be executed with the environment variable `MOCK_API=true` to enable "mocks" mode.

## E2E Tests

`e2e` directory contains end-to-end tests that create a custom resource, operator deployment, required RBAC.

Pre-reqs to run end-to-end (e2e) tests:

* a running OpenShift instance (3.11+)
* current oc/kubectl context as a cluster admin user

### How to build tests binary
```bash
OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $GOPATH/src/github.com/eclipse/che-operator/run-tests $GOPATH/src/github.com/eclipse/che-operator/e2e/*.go
```

Or you can build in a container:

```bash
docker run -ti -v /tmp:/tmp -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.redhat.io/devtools/go-toolset-rhel7:1.11.5-3 sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"
cp /tmp/run-tests ${OPERATOR_REPO}/run-tests
```

### How to run tests using real cluster(e2e)

The resulting binary is created in the root of the repo. Make sure it is run from this location since it uses relative paths to yamls that are then deserialized.
There's a script `run-okd-local.sh` which is more of a CI thing, however, if you can run `oc cluster up` in your environment, you are unlikely to have any issues.

```
./run-tests
```

Tests create a number of k8s/OpenShift objects and generally assume that a fresh installation of OpenShift is available.
TODO: handle AlreadyExists errors to either remove Che namespace or create a new one with a unique name.

### What do tests check?

#### Installation of Che/CRW

A custom resource is created, which signals the operator to deploy Che/CRW with default settings.

#### Configuration changes in runtime

Once an successful installation of Che/CRW is verified, tests patch custom resource to:

* enable oAuth
* enable TLS mode

Subsequent checks verify that the installation is reconfigured, for example uses secure routes or ConfigMap has the right Login-with-OpenShift values

TODO: add more scenarios

### che-operator CI pull request checks

che-operator uses two CI to launch all required checks for operator's pull requests:
  - Github actions CI
  - Openshift CI

Openshift CI configuration defined in the `https://github.com/openshift/release/tree/master/ci-operator/config/eclipse/che-operator` yamls. Openshift ci scripts located in the che-operator repository in the folder `.ci`

Github actions defined in the `.github/workflows` yamls. Scripts located in the `.github/action_scripts`

To relaunch failed Openshift CI checks onto your pull request you can use github comment `/retest`.
To relaunch failed github action checks you have to use github ui(`Re-run jobs` button). 
Unfortunately you can't relanch only one special check for github actions, github doesn't support such a case.

### Check che-operator compilation

To check Che operator compilation you can use command:

```bash
GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go
```

From command you can see the operator will be compiled to binary  `/tmp/che-operator/che-operator`. 
This command is useful to make sure that che-operator is still compiling after your changes.

Also you can use the corresponding VSCode task:`Compile che-operator code`. 

### Format code

To format che-operator execute command:

```bash
$ go fmt ./...
```

Also you can use the VSCode task: `Format che-operator code`.

### Update golang dependencies

che-operator uses go modules and a vendor folder. To update golang dependencies execute command:

```bash
$ go mod vendor
```

Also you can use VSCode task `Update che-operator dependencies`.
New golang dependencies in the vendor folder should be committed and included to the pull request.

## How to add new role/cluster role to che-operator

che-operator uses Kubernetes api to manage Eclipse Che lifecycle.
To request any resource objects from Kubernetes api, che-operator should pass authorization.
Kubernetes api authorization uses role-based access control(RBAC) api(see more https://kubernetes.io/docs/reference/access-authn-authz/rbac/). che-operator has got a service account, like a process inside a pod.
Service account provides identity for che-operator. The service account can be bound to some roles(or cluster roles).
These roles allow che-operator to request some Kubernetes api resources.
Role(or cluster role) bindings we are using to bind roles(or cluster roles) to the che-operator service account.

che-operator roles and cluster roles yaml definition located in the `deploy` folder.
For example `role.yaml`. When you need to provide access to new Kubernetes api part for che-operator, you need
include to this file such information like:

  - api group
  - resource/s name
  - verbs - actions which che-operator can do on this resource/s

When this role/cluster role was updated you should also register `api` cshema in the `pkg/controller/che/che_controller.go` in the `add` method:

```go
import (
  ...
	userv1 "github.com/openshift/api/user/v1"
  ...
)

...
		if err := userv1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift User to scheme: %s", err)
    }
...
```

Then you can use in the che-operator controller code the new Kubernetes api part. When code completed and your pr is ready you should
generate new OLM bundle.
