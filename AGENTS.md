# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Project Overview

Eclipse Che Operator — a Kubernetes/OpenShift operator built with Operator SDK and controller-runtime that manages 
the lifecycle of Eclipse Che installations. It watches `CheCluster` custom resources and reconciles all Che components 
(dashboard, gateway, devfile registry, Che server, etc.).

Go module: `github.com/eclipse-che/che-operator`

## Build & Test Commands

```bash
# Build operator binary
make build

# Run all unit tests
make test

# Run a single test file or package
MOCK_API=true go test -mod=vendor ./controllers/che/... -run TestSpecificName -v

# Run tests for a specific package
MOCK_API=true go test -mod=vendor ./pkg/deploy/gateway/...

# Format code (uses goimports if available, falls back to go fmt)
make fmt

# Run go vet
make vet

# Regenerate CRDs, DeepCopy methods, and related manifests
build/scripts/docker-run.sh make update-dev-resources

# Build operator Docker image
make docker-build IMG=<image>
```

## Single-File Verification

```bash
# Lint a single file
golangci-lint run path/to/file.go

# Lint a single package
go vet ./pkg/deploy/gateway/...

# Type-check a single package (Go compiler is the type-checker)
go build ./path/to/package/...

# Format a single file
goimports -w path/to/file.go

# or if goimports is not installed:
gofmt -w path/to/file.go
```

## Architecture

### CRD & API Versions

- **v2** (`api/v2/`) — current storage version. `CheClusterSpec` has top-level sections: `DevEnvironments`, `Components`, `GitServices`, `Networking`, `ContainerRegistry`.
- **v1** (`api/v1/`) — deprecated, kept for conversion. Files `checluster_conversion_from.go` / `checluster_conversion_to.go` handle round-trip conversion.
- Webhooks (validation, defaulting, conversion) live in `api/v2/checluster_webhook.go`.

After modifying `api/v2/checluster_types.go`, run `make generate` then `make manifests`.

### Controller Structure

**Entry point:** `cmd/main.go` — registers four controllers with the manager:

1. **CheClusterReconciler** (`controllers/che/`) — the primary controller. Watches `CheCluster` CR and orchestrates all component reconciliation.
2. **DevWorkspaceRouting solver** (`controllers/devworkspace/solver/`) — implements `CheRoutingSolver` for DevWorkspace routing.
3. **UserNamespace controller** (`controllers/usernamespace/`) — manages per-user namespace setup.
4. **WorkspaceConfig controller** (`controllers/workspaceconfig/`) — syncs ConfigMaps, Secrets, PVCs, and unstructured objects into workspace namespaces.

### Reconciliation Pipeline

`CheClusterReconciler` uses a `ReconcilerManager` (`pkg/common/reconciler/`) that runs a chain of `Reconcilable` implementations **in order**. Each reconciler returns `(result, done, err)` — the chain stops at the first `done=false`. Registration order in `controllers/che/checluster_controller.go` defines the execution order:

migration → validation → TLS → DevWorkspace config → RBAC → host resolution → postgres → identity provider → registries → editors → dashboard → gateway → Che server → image puller → container capabilities → console link → metrics

### DeployContext

`DeployContext` (`pkg/common/chetypes/types.go`) is the central context object passed to every reconciler. It carries:
- `CheCluster` — the CR being reconciled
- `ClusterAPI` — cached and non-cached k8s clients, discovery client
- `Proxy`, `Authentication` — resolved configuration
- `IsSelfSignedCertificate`, `CheHost`, `DwoNamespace`

### Component Packages (`pkg/deploy/`)

Each Che component has its own package under `pkg/deploy/` (e.g., `dashboard/`, `gateway/`, `postgres/`, `identity-provider/`, `server/`). Each package typically contains:
- A reconciler struct implementing `Reconcilable`
- Kubernetes resource spec builders
- Tests

Helper functions for creating/syncing k8s resources live in `pkg/deploy/` root (e.g., `SyncDeploymentSpecToCluster` in `deployment.go`).

### Platform Detection

`pkg/common/infrastructure/` detects Kubernetes vs OpenShift and feature availability (OAuth, image puller, service monitors). Some reconcilers are conditionally registered based on platform (e.g., `ConsoleLink` and `ContainerCapabilities` are OpenShift-only).

### Operator Defaults

`pkg/common/operator-defaults/` reads default container images and configuration from environment variables. Resource limit/request defaults live in `pkg/common/constants/`.

### Testing

Tests use `MOCK_API=true` to enable a mocked API server. Test helpers in `pkg/common/test/` provide utilities for setting up fake k8s clients and test environments. Tests are co-located with source files (`*_test.go`).

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

## Pattern References

- **New reconciler component:** follow the pattern in `pkg/deploy/dashboard/` — reconciler struct, resource builders, tests
- **New API field:** see `api/v2/checluster_types.go` for examples, then `make generate && make manifests`
- **API conversion (v1↔v2):** follow the pattern in `api/v1/checluster_conversion_to.go` and `api/v1/checluster_conversion_from.go`
- **Syncing resources to workspace namespaces:** follow the pattern in `controllers/workspaceconfig/configmap2sync.go`

## Key Conventions

- Dependencies are vendored (`vendor/`). Commit vendor changes with dependency updates.
- Freeze new transitive dependencies using `replace` directives in `go.mod` to prevent CQ issues.
- License headers are required on all source files — `make license` adds them via `addlicense`.
- Version is in `version/version.go`.

## Red Hat Compliance and Responsible AI Rules

See [redhat-compliance-and-responsible-ai.md](redhat-compliance-and-responsible-ai.md) and the Cursor rules file under `.cursor/rules/`.
