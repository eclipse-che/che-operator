---
paths:
  - "pkg/deploy/**"
  - "controllers/che/**"
---

# Reconciler Rules

- Every component reconciler implements the `Reconcilable` interface (`pkg/common/reconciler/reconcile_manager.go`): `Reconcile()` and `Finalize()`.
- `Reconcile()` returns `(result, done, err)` — return `done=true` only when the component is fully reconciled. The pipeline stops at the first `done=false`.
- Registration order in `controllers/che/checluster_controller.go` matters — reconcilers run sequentially and may depend on earlier ones having completed.
- All reconcilers receive `DeployContext` (`pkg/common/chetypes/types.go`) — use its `ClusterAPI` clients for k8s operations, not standalone clients.
- Use `SyncDeploymentSpecToCluster` and similar helpers in `pkg/deploy/` for creating/updating k8s resources — don't write raw client calls.
- Some reconcilers are OpenShift-only (consolelink, container-capabilities) — guard with `infrastructure.IsOpenShift()`.
- Tests require `MOCK_API=true` environment variable.
