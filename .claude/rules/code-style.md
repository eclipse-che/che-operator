# Code Style Rules

- Add an empty line after logical blocks of code (e.g., after `if` blocks, loops, variable declaration groups) to improve readability.
- Wrap errors with context using `fmt.Errorf` at every call site instead of passing them through directly — this applies both inside the function and at the caller. Error messages must start with `failed to` (lowercase). When the object has a namespace, include it as `namespace/name` in the message. Use `return fmt.Errorf("failed to sync Deployment %s/%s: %w", obj.Namespace, obj.Name, err)` instead of `return err`.
- **Exception**: Do NOT wrap errors when returning `reconcile.Result` from a reconciler's `Reconcile()` method. Return the error directly: `return reconcile.Result{RequeueAfter: 1 * time.Second}, false, err`.
- When referring to Kubernetes resources in error messages or logs, use the capitalized Kind name (e.g., `ClusterRole`, `Deployment`, `Service`) not lowercase variants (e.g., not "cluster role", "deployment", "service").
- Lines should not exceed 120 characters. If a function call exceeds this limit, put each argument on its own line. This does not apply to `fmt.Errorf`, `fmt.Sprintf`, and similar formatting functions — keep those on one line.
- Use `deploy.GetLabels(component)` for labeling resources in `pkg/deploy/` reconcilers.
- Do not use `logrus` for logging. Use the structured JSON logger (`ctrl.Log.WithName(...)`) already defined in the package. If none exists, add one as a package-level `var logger = ctrl.Log.WithName("package-name")`.
- When you have a `DeployContext` parameter (typically named `ctx`), always use `ctx.Context` instead of `context.TODO()` for Kubernetes client operations and other context-aware calls. This ensures proper context propagation for cancellation, deadlines, and tracing. Example: use `clientWrapper.Get(ctx.Context, key, obj)` not `clientWrapper.Get(context.TODO(), key, obj)`.
