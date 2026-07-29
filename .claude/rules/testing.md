# Testing Rules

- Use `test.NewCtxBuilder()` to create test `DeployContext` instances with a fake k8s client.
- Use `infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)` or `infrastructure.Kubernetes` to set the platform. Use OpenShift when testing OpenShift-specific resources (e.g., network policies with `infrastructure.IsOpenShift()` guards).
- Use `defaults.InitializeForTesting("../../config/manager/manager.yaml")` in `init_test.go` (adjust relative path) to load operator defaults for `GetCheFlavor()` and other defaults.
- Tests are co-located with source files (`*_test.go`). Test helpers live in `pkg/common/test/`.
- Use `test.EnsureReconcile(t, ctx, reconciler.Reconcile)` to test reconcilers — it loops up to 10 iterations until `done=true` and fails if the reconciler never finishes.
