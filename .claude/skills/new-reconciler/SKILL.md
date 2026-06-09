---
name: new-reconciler
description: How to add a new component reconciler to the Che operator
---

# Adding a New Reconciler Component

Reference implementation: `pkg/deploy/dashboard/`

## Steps

1. Create a new package under `pkg/deploy/<component>/`
2. Define a reconciler struct implementing `Reconcilable` from `pkg/common/reconciler/`:
   - `Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error)`
   - `Finalize(ctx *chetypes.DeployContext) bool`
3. Build k8s resource specs (Deployment, Service, ConfigMap, etc.) in the same package
4. Register the reconciler in `controllers/che/checluster_controller.go` via `reconcilerManager.AddReconciler()`
5. Placement in the chain matters — add after dependencies are reconciled
6. Run `make test` to verify
