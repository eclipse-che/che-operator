[![Contribute](https://www.eclipse.org/che/contribute.svg)](https://workspaces.openshift.com#https://github.com/eclipse-che/che-operator)
[![Dev](https://img.shields.io/static/v1?label=Open%20in&message=Che%20dogfooding%20server%20(with%20VS%20Code)&logo=eclipseche&color=FDB940&labelColor=525C86)](https://che-dogfooding.apps.che-dev.x6e0.p1.openshiftapps.com#https://github.com/eclipse-che/che-operator)

# Che/Red Hat OpenShift Dev Spaces Operator

[![codecov](https://codecov.io/gh/eclipse-che/che-operator/branch/main/graph/badge.svg?token=IlYvrVU5nB)](https://codecov.io/gh/eclipse-che/che-operator)

- [Description](#Description)
- [Development](#Development)
  - [Update golang dependencies](#Update-golang-dependencies)
  - [Run unit tests](#Run-unit-tests)
  - [Format the code and fix imports](#Format-the-code-and-fix-imports)
  - [Update development resources](#Update-development-resources)
  - [Build Che operator image](#Build-Che-operator-image)
  - [Deploy Che operator](#Deploy-Che-operator)
  - [Update Che operator](#Update-Che-operator)
  - [Debug Che operator](#Debug-Che-operator)
  - [Validation licenses for runtime dependencies](#Validation-licenses-for-runtime-dependencies)
- [Builds](#Builds)
- [License](#License)
 

## Description

Che/Red Hat OpenShift Dev Spaces operator uses [Operator SDK](https://github.com/operator-framework/operator-sdk) and [Go Kube client](https://github.com/kubernetes/client-go) to deploy, update and manage K8S/OpenShift resources that constitute a multi-user Eclipse Che/Red Hat OpenShift Dev Spaces cluster.

The operator watches for a Custom Resource of Kind `CheCluster`, and operator controller executes its business logic when a new Che object is created, namely:

* creates k8s/OpenShift objects
* verifies successful deployment of Postgres, Devfile/Plugin registries, Dashboard and Che server
* updates CR status (passwords, URLs, provisioning statuses etc.)
* etc

Che operator is implemented using [operator framework](https://github.com/operator-framework) and the Go programming language. Eclipse Che configuration defined using a custom resource definition object and stored in the custom Kubernetes resource named `checluster`(or plural `checlusters`) in the Kubernetes API group `org.eclipse.che`. Che operator extends Kubernetes API to embed Eclipse Che to Kubernetes cluster in a native way.

## Development

### Update golang dependencies

```bash
make update-go-dependencies
```

New golang dependencies in the vendor folder should be committed and included in the pull request.

**Note:** freeze all new transitive dependencies using "replaces" in `go.mod` file section
to prevent CQ issues.

### Run unit tests

```bash
make test
```

### Format the code and fix imports

```bash
make fmt
```

### Update development resources

You have to update development resources 
if you updated any files in `config` folder or `api/v2/checluster_types.go` file.
To generate new resource, run the following command:

```bash
make update-dev-resources
```

### Build Che operator image

```bash
make docker-build docker-push IMG=<YOUR_OPERATOR_IMAGE>
```

### Deploy Che operator

For OpenShift cluster:

```bash
build/scripts/olm/test-catalog-from-sources.sh
```

For Kubernetes cluster:

```bash
make gen-chectl-tmpl TEMPLATES=<OPERATOR_RESOURCES_PATH>
chectl server:deploy -p (k8s|minikube|microk8s|docker-desktop) --che-operator-image=<YOUR_OPERATOR_IMAGE> --templates <OPERATOR_RESOURCES_PATH>
```

### Update Che operator

You can modify any Kubernetes object using the UI (for example OpenShift web console) 
or you can also modify Kubernetes objects using the terminal:

```bash
kubectl edit checluster eclipse-che -n <ECLIPSE-CHE-NAMESPACE>
```

or:

```bash
kubectl patch checluster/eclipse-che --type=merge -p '<PATCH_JSON>' -n <ECLIPSE-CHE-NAMESPACE>
```

### Debug Che operator

You can run/debug this operator on your local machine (without deploying to a k8s cluster).


```bash
make debug
```

Then use VSCode debug configuration `Che Operator` to attach to a running process.


### Validation licenses for runtime dependencies

Che operator is an Eclipse Foundation project. So we have to use only open source runtime dependencies with Eclipse compatible license https://www.eclipse.org/legal/licenses.php.
Runtime dependencies license validation process described here: https://www.eclipse.org/projects/handbook/#ip-third-party
To merge code with third party dependencies you have to follow process: https://www.eclipse.org/projects/handbook/#ip-prereq-diligence
When you are using new golang dependencies you have to validate the license for transitive dependencies too.
You can skip license validation for test dependencies.
All new dependencies you can find using git diff in the go.sum file.

Sometimes in the go.sum file you can find few versions for the same dependency:

```go.sum
...
github.com/go-openapi/analysis v0.18.0/go.mod h1:IowGgpVeD0vNm45So8nr+IcQ3pxVtpRoBWb8PVZO0ik=
github.com/go-openapi/analysis v0.19.2/go.mod h1:3P1osvZa9jKjb8ed2TPng3f0i/UY9snX6gxi44djMjk=
...
```

In this case will be used only one version(the highest) in the runtime, so you need to validate license for only one of them(the latest).
But also you can find module path https://golang.org/ref/mod#module-path with major version suffix in the go.sum file:

```go.sum
...
github.com/evanphx/json-patch v4.11.0+incompatible/go.mod h1:50XU6AFN0ol/bzJsmQLiYLvXMP4fmwYFNcr97nuDLSk=
github.com/evanphx/json-patch/v5 v5.1.0/go.mod h1:G79N1coSVB93tBe7j6PhzjmR3/2VvlbKOFpnXhI9Bw4=
...
```

In this case we have the same dependency, but with different major versions suffix.
Main project module uses both these versions in runtime. So both of them should be validated.

Also there is some useful golang commands to take a look full list dependencies:

```bash
$ go list -mod=mod -m all
```

This command returns all test and runtime dependencies. Like mentioned above, you can skip test dependencies.

If you want to know dependencies relation you can build dependencies graph:

```bash
$ go mod graph
```

> IMPORTANT: Dependencies validation information should be stored in the `DEPENDENCIES.md` file.

## Builds

This repo contains several [actions](https://github.com/eclipse-che/che-operator/actions), including:
* [![release latest stable](https://github.com/eclipse-che/che-operator/actions/workflows/release.yml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/release.yml)
* [![next builds](https://github.com/eclipse-che/che-operator/actions/workflows/build-next-images.yaml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/build-next-images.yaml)
* [![PR](https://github.com/eclipse-che/che-operator/actions/workflows/pr-check.yml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/pr-check.yml)

Downstream builds can be found at the link below, which is _internal to Red Hat_. Stable builds can be found by replacing the 3.x with a specific version like 3.2.  

* [operator_3.x](https://main-jenkins-csb-crwqe.apps.ocp-c1.prod.psi.redhat.com/job/DS_CI/job/operator_3.x/)
* [operator-bundle_3.x](https://main-jenkins-csb-crwqe.apps.ocp-c1.prod.psi.redhat.com/job/DS_CI/job/operator-bundle_3.x/)

See also:
* [dsc_3.x](https://main-jenkins-csb-crwqe.apps.ocp-c1.prod.psi.redhat.com/job/DS_CI/job/dsc_3.x) (downstream equivalent of [chectl](https://github.com/redhat-developer/devspaces-chectl/))


## License

Che is open sourced under the Eclipse Public License 2.0.
