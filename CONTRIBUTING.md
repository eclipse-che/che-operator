# Contributing to Eclipse Che Operator

- [Quick Start](#quick-start)
- [Run unit tests](#run-unit-tests)
- [Format the code and fix imports](#format-the-code-and-fix-imports)
- [Run static code analyzers](#run-static-code-analyzers)
- [Update development resources](#update-development-resources)
- [Build Che operator image](#build-che-operator-image)
- [Deploy Che operator from the source code](#deploy-che-operator-from-the-source-code)
- [Debug Che operator](#debug-che-operator)
- [Pull requests](#pull-requests)

### Quick Start

```bash
git clone https://github.com/eclipse-che/che-operator.git
cd che-operator
make build  # Build the operator binary
make test   # Run unit tests to verify setup
```

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
make update-dev-resources
```

On macOS, run it inside a container:

```bash
build/scripts/docker-run.sh make update-dev-resources
```

### Build Che operator image

```bash
make docker-build IMG=<YOUR_OPERATOR_IMAGE>
```

### Deploy Che operator from the source code

For OpenShift cluster:

```bash
build/scripts/oc-tests/oc-test.sh --install-from sources=next
```

For Kubernetes cluster:

```bash
build/scripts/minikube-tests/test-operator-from-sources.sh
```

### Debug Che operator

You can run/debug this operator on your local machine (without deploying to a k8s cluster).

```bash
make debug
```

Then use VSCode debug configuration `Che Operator` to attach to a running process.

### Pull requests

Use the [pull request template](.github/PULL_REQUEST_TEMPLATE.md) and fill in all sections, including steps to deploy and test the change.
See also [Red Hat compliance and responsible AI](redhat-compliance-and-responsible-ai.md).
