# AGENTS.md

This file provides architectural context for AI coding agents working in this repository.
Actionable rules (build commands, code style, K8s client usage, compliance) live in `.claude/rules/`.

## Project Overview

Eclipse Che Operator — a Kubernetes/OpenShift operator built with Operator SDK and controller-runtime that manages
the lifecycle of Eclipse Che installations. It watches `CheCluster` custom resources and reconciles all Che components
(dashboard, gateway, devfile registry, Che server, etc.).

Go module: `github.com/eclipse-che/che-operator`

## Architecture

### CRD & API Versions

- **v2** (`api/v2/`) — current storage version. `CheClusterSpec` has top-level sections: `DevEnvironments`, `Components`, `GitServices`, `Networking`, `ContainerRegistry`.
- **v1** (`api/v1/`) — deprecated, exists only for conversion compatibility.
- Webhooks (validation, defaulting, conversion) live in `api/v2/checluster_webhook.go`.

### Controller Structure

**Entry point:** `cmd/main.go` — registers four controllers with the manager:

1. **CheClusterReconciler** (`controllers/che/`) — the primary controller. Watches `CheCluster` CR and orchestrates all component reconciliation.
2. **DevWorkspaceRouting solver** (`controllers/devworkspace/solver/`) — implements `CheRoutingSolver` for DevWorkspace routing.
3. **UserNamespace controller** (`controllers/usernamespace/`) — manages per-user namespace setup.
4. **WorkspaceConfig controller** (`controllers/workspaceconfig/`) — syncs ConfigMaps, Secrets, PVCs, and unstructured objects into workspace namespaces.

### Reconciliation Pipeline

`CheClusterReconciler` uses a `ReconcilerManager` (`pkg/common/reconciler/`) that runs a chain of
`Reconcilable` implementations **in order**. Each reconciler returns `(result, done, err)` — the chain stops
at the first `done=false`. Registration order in `controllers/che/checluster_controller.go` defines the execution order.

### DeployContext

`DeployContext` (`pkg/common/chetypes/types.go`) is the central context object passed to every reconciler. It carries:
- `CheCluster` — the CR being reconciled
- `ClusterAPI` — cached and non-cached k8s clients, discovery client, `ClientWrapper` / `NonCachingClientWrapper`
- `Proxy`, `Authentication` — resolved configuration
- `IsSelfSignedCertificate`, `CheHost`, `DwoNamespace`

### Component Packages (`pkg/deploy/`)

Each Che component has its own package under `pkg/deploy/` (e.g., `dashboard/`, `gateway/`, `postgres/`, `identity-provider/`, `server/`). Each package typically contains:
- A reconciler struct implementing `Reconcilable`
- Kubernetes resource spec builders
- Tests

### Platform Detection

`pkg/common/infrastructure/` detects Kubernetes vs OpenShift and feature availability (OAuth, image puller, service monitors).
Some reconcilers are conditionally registered based on platform (e.g., `ConsoleLink` and `ContainerCapabilities` are OpenShift-only).

### Operator Defaults

`pkg/common/operator-defaults/` reads default container images and configuration from environment variables.
Resource limit/request defaults live in `pkg/common/constants/`.

### Testing

Test helpers in `pkg/common/test/` provide utilities for setting up fake k8s clients and test environments. Tests are co-located with source files (`*_test.go`).

### OLM & Deployment

- **OLM bundles:** `bundle/` (per-channel: `next`, `stable`)
- **OLM catalog:** `olm-catalog/` (per-channel with `index.Dockerfile`)
- **Helm charts:** `helmcharts/`
- **Kustomize overlays:** `config/` (separate overlays for `kubernetes/` and `openshift/`)
- **Generated deployment YAMLs:** `deploy/deployment/` (created by `make gen-deployment`)
- **Editor definitions:** `editors-definitions/` — YAML definitions for IDE editors bundled into the operator image

### Deploy for Testing

```bash
# OpenShift — deploy from sources via OLM
build/scripts/olm/test-catalog-from-sources.sh

# Minikube
build/scripts/minikube-tests/test-operator-from-sources.sh
```
