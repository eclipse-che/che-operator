## Che/CodeReady Workspaces Operator

Che/CodeReady workspaces operator uses [Operator SDK](https://github.com/operator-framework/operator-sdk) and [Go Kube client](https://github.com/kubernetes/client-go) to deploy, update and manage K8S/OpenShift resources that constitute a multi user Eclipse Che/CodeReady Workspaces cluster.

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

## Project State: Beta

The project is in its early development and breaking changes are possible.

## How to Deploy

**IMPORTANT! Cluster Admin privileges are required**

```
./deploy.sh $namespace
```

The script will create sa, role, role binding, operator deployment, CRD and CR.

Wait until Che deployment is scaled to 1 and Che route is created.

When on pure k8s, make sure you provide a global ingress domain in `deploy/crds/org_v1_che_cr.yaml` for example:

```bash
  k8s:
    ingressDomain: '192.168.99.101.nip.io'
```

### OpenShift oAuth

Bear in mind that che-operator service account needs to have cluster admin privileges so that the operator can create oauthclient at a cluster scope.
There is `oc adm` command in both deploy scripts. Uncomment it if you need this feature.
Make sure your current user has cluster-admin privileges.

### TLS

#### OpenShift

When using self-signed certificates make sure you set `server.selfSignedCert` to true
or create a secret called `self-signed-certificate` in a target namespace with ca.crt holding your OpenShift router crt body.
When `server.selfSignedCert` the operator will create a test TLS route, GET it, extract certificate chain, convert to a secret `self-signed-certificate`,
and Che/CRW server will automatically add it to Java trust store.

#### K8S

When enabling TLS, make sure you create a secret with crt and key, and let the Operator know about it in `k8s.tlsSecretName`

## How to Configure

The operator watches all objects it creates and reconciles them with CR state. It means that if you edit a configMap **che**, the operator will revert changes.
Since not all Che configuration properties are custom resource spec fields (there are simply too many of them), the operator creates a second configMap called **custom**
which you can use for any environment variables not supported by CR field. The operator will not reconcile configMap custom.

## How to Build Operator Image

```bash
docker build -t $registry/$repo:$tag
```

You can then use the resulting image in operator deployment (deploy/operator.yaml)

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

### Pre-Reqs: /tmp/keycloak_provision file

The operator grabs this file and replaces values to get a string used as exec command to create Keycloak realm, client and user.
Make sure you run the following before running/debugging:

```
cp deploy/keycloak_provision /tmp/keycloak_provision
```
This file is added to a Docker image, thus this step isn't required when deploying an operator image.

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




   
