[![Contribute](https://www.eclipse.org/che/contribute.svg)](https://workspaces.openshift.com#https://github.com/eclipse-che/che-operator)
[![Dev](https://img.shields.io/static/v1?label=Open%20in&message=Che%20dogfooding%20server%20(with%20VS%20Code)&logo=eclipseche&color=FDB940&labelColor=525C86)](https://che-dogfooding.apps.che-dev.x6e0.p1.openshiftapps.com#https://github.com/eclipse-che/che-operator)

# Eclipse Che Operator

[![codecov](https://codecov.io/gh/eclipse-che/che-operator/branch/main/graph/badge.svg?token=IlYvrVU5nB)](https://codecov.io/gh/eclipse-che/che-operator)

- [Description](#Description)
- [Development](#Development)
  - [Quick Start](#Quick-Start)
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

Eclipse Che operator uses [Operator SDK](https://github.com/operator-framework/operator-sdk) and [Go Kube client](https://github.com/kubernetes/client-go) to deploy, update
and manage Kubernetes/OpenShift resources that constitute an Eclipse Che cluster.

Che operator is implemented using [operator framework](https://github.com/operator-framework) and the Go programming language. 
Eclipse Che configuration is defined using a custom resource definition object and stored in the custom Kubernetes resource named 
`checluster` (or plural `checlusters`) in the Kubernetes API group `org.eclipse.che`. 

## Development

### Quick Start

```bash
git clone https://github.com/eclipse-che/che-operator.git
cd che-operator
make build  # Build the operator binary
make test   # Run unit tests to verify setup
```

### Update golang dependencies

```bash
make update-go-dependencies
```

New golang dependencies in the vendor folder should be committed and included in the pull request.

### Run unit tests

```bash
make test
```

### Format the code and fix imports

```bash
make fmt
```

### Run static code analyzers

```bash
make lint
```

### Update development resources

You have to update development resources if you updated any files in `config` folder or `api/v2/checluster_types.go` file:

```bash
build/scripts/docker-run.sh make update-dev-resources
```

### Build Che operator image

```bash
make docker-build IMG=<YOUR_OPERATOR_IMAGE>
```

### Deploy Che operator from the source code:

For OpenShift cluster:

```bash
build/scripts/docker-run.sh /bin/bash -c "
  oc login \
    --token=<...> \
    --server=<...> \
    --insecure-skip-tls-verify=true && \
  build/scripts/olm/test-catalog-from-sources.sh
"
```

For Kubernetes cluster:

```bash
./build/scripts/minikube-tests/test-operator-from-sources.sh
```

### Debug Che operator

You can run/debug this operator on your local machine (without deploying to a k8s cluster).

```bash
make debug
```

Then use VSCode debug configuration `Che Operator` to attach to a running process.

### Validation licenses for runtime dependencies

Che operator is an Eclipse Foundation project. 
So we have to use only open source runtime dependencies with Eclipse compatible license https://www.eclipse.org/legal/licenses.php.
Runtime dependencies license validation process described here: https://www.eclipse.org/projects/handbook/#ip-third-party
To merge code with third party dependencies you have to follow process: https://www.eclipse.org/projects/handbook/#ip-prereq-diligence
When you are using new golang dependencies you have to validate the license for transitive dependencies too.

## Builds

This repo contains several [actions](https://github.com/eclipse-che/che-operator/actions), including:
* [![release latest stable](https://github.com/eclipse-che/che-operator/actions/workflows/release.yml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/release.yml)
* [![next builds](https://github.com/eclipse-che/che-operator/actions/workflows/build-next-images.yaml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/build-next-images.yaml)
* [![PR](https://github.com/eclipse-che/che-operator/actions/workflows/pr-check.yml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/pr-check.yml)

## License

Che is open sourced under the Eclipse Public License 2.0.
