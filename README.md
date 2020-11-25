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

## How to Deploy

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

### Build custom che-operator image
You can build custom che-operator image using command:

```bash
$ 
```

> Notice: you can use VSCode task `Build and push custom che-operator image`. But you need to specify
env variables: IMAGE_REGISTRY_HOST, IMAGE_REGISTRY_USER_NAME(for regular development you can use .bashrc for this purpose).
This task will build and push image with name `${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly`
to the registry.

### Test che-operator with chectl via `operator` installer
If you want to test modified che-operator using chectl, you have to build custom che-operator image.



Launch test cluster. Install Eclipse Che:

```bash
$ chectl deploy:server -n che --installer operator -p ${platform} --che-operator-image=${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly
```

where is `platform` it's a cluster platform.

> INFO: if you changed che-operator deployment or role/cluster role, CRD, CR you have to provide `--templates` argument.
This argument will points chectl to your modificated che-operator `deploy` folder path.

### How to test che-operator with chectl via OLM

The following instructions show how to test Che operator under development using OLM installer.

1. Build your custom operator image and use it in the operator deployment: [How to Build Operator Image](#how-to-build-operator-image)).
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
  - `IMAGE_REGISTRY_HOST` - host of the image registry, for example: "docker.io", "quay.io". Host could be with non default port: localhost:5000, 127.0.0.1:3000 and etc.

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
Replace value of `image` field with your catalog source image. Don't forget to specify desired platform.

5. Deploy Che using chectl:
```sh
$ chectl server:deploy --installer=olm --platform=${platform} -n ${che-namespace} --catalog-source-yaml ${path_to_custom_catalog_source_yaml} --olm-channel=nightly --package-manifest-name=eclipse-che-preview-${platform}
```

### OpenShift oAuth

Bear in mind that che-operator service account needs to have cluster admin privileges so that the operator can create oauthclient at a cluster scope.
There is `oc adm` command in both deploy scripts. Uncomment it if you need this feature.
Make sure your current user has cluster-admin privileges.

### TLS

TLS is enabled by default.
Turning it off is not recommended as it will cause malfunction of some components.

#### OpenShift

When the cluster is configured to use self-signed certificates for the router, the certificate will be automatically propogated to Che components as trusted.
If cluster router uses certificate signed by self-signed one, then parent/root CA certificate should be added into corresponding config map of additional trusted certificates (see `serverTrustStoreConfigMapName` option).

#### K8S

By default self-signed certificates for Che will be generated automatically.
If it is needed to use own certificates, create `che-tls` secret (see `k8s.tlsSecretName` option) with `key.crt` and `tls.crt` fields. In case of self-signed certificate `self-signed-certificate` secret should be created with public part of CA certificate under `ca.crt` key in secret data.

## How to Configure

The operator watches all objects it creates and reconciles them with CR state. It means that if you edit a configMap **che**, the operator will revert changes.
Since not all Che configuration properties are custom resource spec fields (there are simply too many of them), the operator creates a second configMap called **custom**
which you can use for any environment variables not supported by CR field. The operator will not reconcile configMap custom.

## How to Build Operator Image
In the root of the che-operator project:

```bash
docker build -t $registry/$repo:$tag .
```

You can then use the resulting image in operator deployment (deploy/operator.yaml): replace default operator image `quay.io/eclipse/che-operator:nightly` with yours (say, `docker.io/user/che-operator:latest`)

## Build and Deploy to a local cluster:

There's a little script that will build a local Docker image and deploy an operator to a selected namespace,
as well as create service account, role, role binding, CRD and example CR.

```
oc new-project $namespace
build_deploy_local.sh $namespace

```

The above method will work only with Docker 17.x (does not works if you want to build in MiniShift/MiniKube). Mostly useful if you run `oc cluster up` locally.

## How to Run/Debug Locally

You can run/debug this operator on your local machine (not deployed to a k8s cluster),
provided that the below pre-reqs are met.

### Pre-Reqs: Local kubeconfig
Go client grabs kubeconfig either from InClusterConfig or ~/.kube locally.
Make sure you oc login (or your current kubectl context points to a target cluster and namespace),
and current user/server account can create objects in a target namespace.

### Pre-Reqs: WATCH_NAMESPACE Environment Variable

The operator detects namespace to watch by getting value of `WATCH_NAMESPACE` environment variable.
You can set it in Run configuration in the IDE, or export this env before executing the binary.

This applies both to Run and Debug.

### Pre-Reqs: /tmp/keycloak_provision and oauth_provision files

The Operator takes these files and replaces values to get a string used as the `exec` command to configure Keycloak.
Make sure you run the following before running/debugging:

```
cp templates/keycloak_provision /tmp/keycloak_provision
cp templates/oauth_provision /tmp/oauth_provision
```
These files are added to a container image, and thus this step is not required when deploying an Operator image.

## E2E Tests

`e2e` directory contains end-to-end tests that create a custom resource, operator deployment, required RBAC.

Pre-reqs to run end-to-end (e2e) tests:

* a running OpenShift instance (3.11+)
* current oc/kubectl context as a cluster admin user

### How to build tests binary
```
OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $GOPATH/src/github.com/eclipse/che-operator/run-tests $GOPATH/src/github.com/eclipse/che-operator/e2e/*.go
```

Or you can build in a container:

```
docker run -ti -v /tmp:/tmp -v ${OPERATOR_REPO}:/opt/app-root/src/go/src/github.com/eclipse/che-operator registry.redhat.io/devtools/go-toolset-rhel7:1.11.5-3 sh -c "OOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/run-tests /opt/app-root/src/go/src/github.com/eclipse/che-operator/e2e/*.go"
cp /tmp/run-tests ${OPERATOR_REPO}/run-tests
```

### How to run tests

The resulted binary is created in the root of the repo. Make sure it is run from this location since it uses relative paths to yamls that are then deserialized.
There's a script `run-okd-local.sh` which is more of a CI thing, however, if you can run `oc cluster up` in your environment, you are unlikely to have any issues.

```
./run-tests
```

Tests create a number of k8s/OpenShift objects and generally assume that a fresh installation of OpenShift is available.
TODO: handle AlreadyExists errors to either remove che namespace or create a new one with a unique name.

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
  - github actions CI
  - openshift CI

Openshift CI configuration defined in the `https://github.com/openshift/release/tree/master/ci-operator/config/eclipse/che-operator` yamls. openshift ci scripts located in the che-operator repository in the folder `.ci`

Github actions defined in the `.github/workflows` yamls. Scripts located in the `.github/action_scripts`

To relaunch failed openshift CI checks onto your pull request you can use github comment `/retest`.
To relaunch failed github action checks you have to use github ui(`Re-run jobs` button). 
Unfortunately you can't relanch only one special check for github actions, github doesn't support such case.

### Debug test scripts

To debug test scripts you can use the "Bash debug" VSCode extension. 
For a lot of test scripts you can find different debug configurations in the `.vscode/launch.json`.

### Local debug che-operator(outside cluster)

To run che-operator in debug mode you can use `local-debug.sh` script. After process execution you should attach IDE debugger to debug port: 2345. Also you can see this port number in the process output.
`local-debug.sh` script has got two arguments:
 - `-n` - Eclipse Che installation namespace. Default value is `che`.
 - `-cr` - path to the custom resource yaml definition. Default value points to file `./deploy/crds/org_v1_che_cr.yaml`.
For VSCode we have launch.json configuration `Che Operator`. So in the VSCode debugger you can find this debug
configuration, select it, click `Start debugging` button. VSCode will attach debugger to che-operator process.

### Check che-operator compilation

To check Che operator compilation you can use command:

```bash
GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go
```

From command you can see operator will be compiled
to binary to folder `/tmp/che-operator/che-operator`. 
This command is usefull to make sure that che-opeator is still compiling after your changes.

Also we have corresponding VSCode task:`Check Che operator compilation`. 

### Format code

Use command to format code:


### Update golang dependencies
Todo


### 18. Debug che-operator tests in VSCode 

To debug che-operator tests you can use VSCode `Launch Current File` debug configuration.
For that you have to open file with test, for example `pkg/controller/che/che_controller_test.go`,
set up some breakpoints, select debug tab, select `Launch Current File` configuration in the debug panel
and click `Start debugging` button. Test will be executed with env variable `` to enable "mocks" mode.

## How to add new role/cluster role to che-operator

che-operator uses kubernetes api to manage Eclipse Che lifecycle.
To request any resouce objects from kubernetes api, che-operator should pass authorization.
Kubernetes api authorization uses role-based access control(RBAC) api(see more https://kubernetes.io/docs/reference/access-authn-authz/rbac/). che-operator has got service account, like process inside a pod.
Service account provides identity for che-operator. To service account can be bound some roles(or cluster roles).
This roles allows che-operator to request some kubernetes api resources.
Role(or cluster role) bindings we are using to bind roles(or cluster roles) to che-operator service account.

che-operator roles and cluster roles yaml definition located in the `deploy` folder.
For example `role.yaml`. When you need to provide access to new kubernetes api part for che-operator, you need
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

Then you can use in che-operator controller code new kubernetes api part. When code complited and your pr is ready you should
generate new OLM bundle.

### Using chectl to test che-operator

To test che-operator changes you can use chectl: https://github.com/che-incubator/chectl
chectl has got two installer types corresponding to che-operator: `operator` and `olm`.
With `operator` installer chectl reuses copies of che-operator deployment and role(cluster role) yamls from folder `deploy`.
With `olm` installer chectl uses catalog source index image with olm bundles from `deploy/olm-catalog`.
chectl supports cluster platforms: "minikube", "minishift", "k8s", "openshift", "microk8s", "docker-desktop", "crc".


### Test che-operator with chectl and `olm` installer
todo

### Test che-operator with chectl and cr patch file
todo




