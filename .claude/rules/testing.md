# Testing Rules

Test helpers live in `pkg/common/test` (package `test`). Tests are co-located with source files (`*_test.go`).

## Setup

- Call `test.StartOn(infrastructure.OpenShiftV4)` or `test.StartOn(infrastructure.Kubernetes)` at the start of each test (or test case). It resets the fake `clusterapi` clients and initializes platform detection (Che and DevWorkspace) and operator defaults (`GetCheFlavor()` etc.). Use OpenShift when testing OpenShift-specific resources (e.g., network policies with `infrastructure.IsOpenShift()` guards).
- Do not add `init_test.go` files that initialize defaults or the platform — `test.StartOn` replaces them.
- Call `test.StartOn` **before** constructing reconcilers or `CheContext`s. Reconcilers (`NewReconciler`, `NewCheUserNamespaceReconciler`, `NewWorkspacesConfigReconciler`) snapshot `clusterapi.Get()` at construction, and `test.StartOn` replaces the global — a reconciler built before `StartOn` keeps the old, empty fake client and fails with confusing not-found errors. Never construct reconcilers at package level or in `TestMain`.
- Do not use `t.Parallel()` in suites that call `test.StartOn`: it swaps the global `clusterapi` clients, which would race with concurrently running tests.
- For envtest-based suites (ginkgo/gomega), start the control plane with `env = envtest.Start()` (from `pkg/common/test/envtest`) in `BeforeSuite` and call `env.Stop()` in `AfterSuite`. `envtest.Start()` initializes `clusterapi` with real clients, so use `test.InitializeDefaults(infra)` instead of `test.StartOn` (which would replace them with fakes). `test.InitializeDefaults` is also useful when only defaults are needed without resetting the clients (e.g., computing expected values before per-case `test.StartOn` calls).

## CheContext

- Use `test.NewCheContextBuilder()` to create test `CheContext` instances backed by `clusterapi.Get()`.
  - By default it contains a CheCluster `eclipse-che/eclipse-che`. Use `WithCheCluster(cheCluster)` to provide a custom one, or `WithEmptyCheCluster()` for a CheCluster without `TypeMeta`.
  - Objects passed via `WithObjects(...)` (and the CheCluster) are created in the cluster on `Build()`, together with their namespaces.

## Assertions

- Use `assert.NoError(t, test.ReconcileTwice(cheCtx, reconciler.Reconcile))` (testify) or `Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())` (gomega) to test reconcilers — the reconciler must return `done=true` with an empty `Result` within two calls.
- Use `test.ExpectFound(key, obj)` / `test.ExpectNotFound(key, obj)` to check whether an object exists, e.g. `assert.NoError(t, test.ExpectFound(key, &corev1.Service{}))`. `ExpectFound` also populates `obj` for further assertions.
