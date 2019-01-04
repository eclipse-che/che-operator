# Che/Codeready Operator

Che Operator creates Eclipse Che k8s and OpenShift resources such as pvcs, services, deployments, routes, ingresses etc.

The operator is a k8s a pod that runs an image with Go runtime and a compiled binary of an operator itself.

Though operator-sdk framework is used, Che operator is rather an installer since no CRD and API group are created,
and thus, the operator does not watch resources. Once deployment is completed, the operator pod exits.

## Pre-Reqs

OpenShift/K8S cluster with at least 4GB or RAM and 2 PVs, local `oc` or `kubectl`.

## How to deploy

Deploy script will create a namespace, operator service account and a rolebinding for it (admin privileges within the namespace),
and run an operator pod that will create all required objects and perform provisioning:

```bash
deploy/deploy.sh $infra $namespace
```

Deploy to MiniKube to a default namespace:

```bash
deploy/deploy.sh minikube

```
`minikube` as infra argument will instruct the script to detect MiniKube IP and pass global ingress domain value to the operator.

Create default namespace eclipse-che and deploy to k8s:

```
deploy/deploy.sh k8s
```

Create a default eclipse-che namespace and deploy to OpenShift:

```
deploy/deploy.sh
```

Create a namespace of choice and deploy to OpenShift:

```
deploy/deploy.sh openshift myproject
```

If a namespace cannot be created (for example, it exists), a new namespace with a generated name (`eclipse-che${random int}`) will be created.

This will deploy Che operator with the default settings:

* upstream Che
* no tls
* no login with OpenShift in Che
* Postgres passwords are auto-generated, Keycloak admin password is `admin`
* Some object names are default ones (eg databases, users etc)
* Common PVC strategy (all workspaces use one shared PVC)
* All workspace objects get created in a target namespace (Che server uses service account token)
* Multi-host ingress strategy when on k8s

## Deploy custom image

Provide your own image in `deploy/config.yaml`:

```shell
CHE_IMAGE: "myRegistry/myImage:myTag"
```

## Deploy CodeReady Workspaces

In `deploy/config.yaml`:

```
CHE_FLAVOR: "codeready"
```
Che flavor is a string that will be used in names of certain objects like routes, realms, clients etc.

## Defaults and Configuration

To deploy to OpenShift with all defaults, no user input is required. You may configure Che installation in a configmap `deploy/config.yaml`.
The operator will use envs from this configmap and make decisions accordingly.

Currently, only the most critical envs are added to configmap, and it will expand in time, making it possible to fine tune Che before deploying

## What is deployed?

The Operator creates a handful of objects for:

* Postgres DB
* Keycloak/Red Hat SSO
* Che server itself

After Postgres and Keycloak pods start and health checks confirm the services are up, the operator run execs into db and keycloak pods to provision
databases, users, Keycloak realm, client and user. The operator watches respective deployments to get events signalling that the deployment is scaled to 1.

## How to configure installation

`deploy/config.yaml` is a config map with env variables that influences choices an operator makes. Each env is commented, and each one has defaults.
This configmap is an operator's env, and evn variables are then taken to Che server configmap.

What can be configured:

* external DB and Keycloak: `CHE_EXTERNAL_DB` and `CHE_EXTERNAL_KEYCLOAK` default to false.
If you do not need instances of Postgres and Keycloak and want to connect to own infra, set both envs to `true` and provide connection details in envs below the above booleans.

Your DB user **MUST** be a `SUPERUSER`.

**Important!** The operator does not perform Postgres and Keycloak provisioning if external instances are used.
Thus, you need to pre-create db and user for Che.
Also create (or use existing) realm and client that should be public and should have:

Redirect URIs: `${PROTOCOL}://${CHE_HOST}/*`
WebOrigins: `${PROTOCOL}://${CHE_HOST}`

* Login with OpenShift in Che. Not supported on k8s infra

* TLS. Set `TLS_SUPPORT` to true if you want to deploy Che in https mode.
When on k8s, make sure you create a secret in che namespace and provide its name in `TLS_SECRET_NAME`

* TLS and self signed certs. Provide a base64 string of your self signed cert in `SELF_SIGNED_CERT`. When on k8s, you can get it from crt part of the secret.

* Fake dns. If, for example, you want to deploy on k8s with a fake ingress domain of example.com and you need to point it to your Minikube IP, `HOST_ALIAS_IP` and `HOST_ALIAS_HOSTNAME` are the two envs to use.

## How to build and deploy own operator image

### Build an image

To build an Operator and package it into a Docker image run:

`docker build -t che-operator .`

If you want to deploy your custom Operator image to a remote k8s/OpenShift cluster, make sure you push an image and change its name in
`--image` and `--overrides` in kubectl/oc run command in the script.

### Build and deploy to a local cluster

Run `deploy/build_deploy_local.sh $infra $namespace`

Minishift and Minikube users will need to execute the below command to use VM Docker daemon when building an image:

```
eval $(minikube docker-env)
```

## How to add new envs to configmap?

If you need to add new envs to be added to Che configmap by default:

* add key:value to deploy/config.yaml
* add operator variable to `pkg/operator/vars.go` that will take its value from environment
* add env and operator variable as value to `pkg/operator/che_cm.go`

## Deploy Script

It is something quick and dirty and is likely to be substituted with a feature rich CLI.