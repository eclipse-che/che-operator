[![Contribute](https://www.eclipse.org/che/contribute.svg)](https://workspaces.openshift.com#https://github.com/eclipse-che/che-operator)
[![Dev](https://img.shields.io/static/v1?label=Open%20in&message=Che%20dogfooding%20server%20(with%20VS%20Code)&logo=eclipseche&color=FDB940&labelColor=525C86)](https://che-dogfooding.apps.che-dev.x6e0.p1.openshiftapps.com#https://github.com/eclipse-che/che-operator)

# Eclipse Che Operator

[![codecov](https://codecov.io/gh/eclipse-che/che-operator/branch/main/graph/badge.svg?token=IlYvrVU5nB)](https://codecov.io/gh/eclipse-che/che-operator)

- [Description](#description)
- [Installation](#installation)
- [Development](#development)
- [Builds](#builds)
- [License](#license)

## Description

Eclipse Che operator installs, configures, upgrades and manages [Eclipse Che](https://github.com/eclipse-che/che) on Kubernetes and OpenShift.
It is written in Go using [Operator SDK](https://github.com/operator-framework/operator-sdk) and [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime).

Eclipse Che configuration is defined by the `CheCluster` custom resource (`checlusters.org.eclipse.che`, storage version `v2`).

Cloud development environments are run by the [DevWorkspace Operator](https://github.com/devfile/devworkspace-operator);
Che operator configures it and prepares user namespaces with the ConfigMaps, Secrets, PersistentVolumeClaims
and other objects that workspaces need.

## Installation

To install Eclipse Che, follow the [Eclipse Che documentation](https://eclipse.dev/che/docs/).

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) to build, test, deploy and debug the operator from the source code.

## Builds

This repo contains several [actions](https://github.com/eclipse-che/che-operator/actions), including:
* [![Stable release](https://github.com/eclipse-che/che-operator/actions/workflows/release.yml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/release.yml)
* [![Next release](https://github.com/eclipse-che/che-operator/actions/workflows/release-next-catalog-and-operator-image.yaml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/release-next-catalog-and-operator-image.yaml)
* [![PR](https://github.com/eclipse-che/che-operator/actions/workflows/pr-check.yml/badge.svg)](https://github.com/eclipse-che/che-operator/actions/workflows/pr-check.yml)

## License

Che is open sourced under the Eclipse Public License 2.0.
