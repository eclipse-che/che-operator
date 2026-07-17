# Testing Rules

- Use `test.NewCtxBuilder()` to create test `DeployContext` instances with a fake k8s client.
- For OpenShift tests, register both `corev1.Namespace` and `projectv1.Project` for each namespace, plus a `configv1.Proxy{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}` — the reconciler reads the Project (not Namespace) on OpenShift, and some code paths read the cluster proxy.
- Use `infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)` or `infrastructure.Kubernetes` to set the platform. Use OpenShift when testing OpenShift-specific resources (e.g., network policies with `infrastructure.IsOpenShift()` guards).
- Use `defaults.InitializeForTesting("../../config/manager/manager.yaml")` in `init_test.go` (adjust relative path) to load operator defaults for `GetCheFlavor()` and other defaults.
- Tests are co-located with source files (`*_test.go`). Test helpers live in `pkg/common/test/`.
