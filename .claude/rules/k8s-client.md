# K8s Client Rules

- Use the `K8sClient` wrapper (`pkg/common/k8s-client/`) for all Kubernetes operations.
- Access it via `DeployContext.ClusterAPI.ClientWrapper` (cached) or `DeployContext.ClusterAPI.NonCachingClientWrapper` (non-caching).
- In the usernamespace controller, use the `r.clientWrapper` / `r.nonCachedClientWrapper` fields directly.
- **Do NOT use** the legacy functions from `pkg/deploy/sync.go` (`deploy.Sync`, `deploy.Get*`, `deploy.Delete*`, `deploy.CreateIgnoreIfExists`) — that file is deprecated.
- When using `Sync` to create/update an object in the **same namespace** as the CheCluster CR, set the owner reference before syncing: `controllerutil.SetControllerReference(ctx.CheCluster, obj, ctx.ClusterAPI.Scheme)`. Do **not** set owner references on objects in other namespaces (cross-namespace owner references are not supported by Kubernetes).
- When using `Sync`, always pass `&k8sclient.SyncOptions{DiffOpts: diffs.<Type>}` to control which fields are compared. Diff options live in `pkg/common/diffs/diffs.go`. If no diff option exists for the resource type, add one there first.
- When deleting, use `DeleteByKeyIgnoreNotFound` for idempotent cleanup (e.g., toggling a feature off).
