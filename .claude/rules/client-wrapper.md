# Client Wrapper Rules

- Use the `ClientWrapper` interface (`pkg/provision/clientwrapper`) for all Kubernetes operations. It is implemented by `K8sClientWrapper`.
- Access it via `CheContext.ClusterAPI.ClientWrapper` (cached) or `CheContext.ClusterAPI.NonCachingClientWrapper` (non-caching).
- When using `ClientWrapper.Sync` to create/update an object in the **same namespace** as the CheCluster CR, set the owner reference before syncing: `controllerutil.SetControllerReference(cheCtx.CheCluster, obj, cheCtx.ClusterAPI.Scheme)`. Do **not** set owner references on objects in other namespaces (cross-namespace owner references are not supported by Kubernetes).
- When using `ClientWrapper.Sync`, always pass `&clientwrapper.SyncOptions{DiffOpts: diffs.<Type>}` to control which fields are compared. Diff options live in `pkg/common/diffs/diffs.go`. If no diff option exists for the resource type, add one there first.
- When reading objects, use `ClientWrapper.GetIgnoreNotFound` — do **not** use `Client.Get`. `GetIgnoreNotFound` returns `(found bool, err error)` instead of returning a "not found" error.
- When deleting, use `DeleteByKeyIgnoreNotFound` for idempotent cleanup (e.g., toggling a feature off).
- For a higher-level alternative to calling `ClientWrapper.Sync` directly, prefer `syncher.SyncObject` (`pkg/provision/syncher`): it sets the CheCluster owner reference automatically, merges existing labels/annotations instead of replacing them, and picks the cached or non-caching wrapper based on the object's labels. Note that it builds its own diff options and must **not** be combined with `pkg/common/diffs/diffs.go`.
