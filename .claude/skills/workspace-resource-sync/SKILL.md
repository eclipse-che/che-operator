---
name: workspace-resource-sync
description: How to sync resources into workspace namespaces
---

# Syncing Resources to Workspace Namespaces

Reference implementation: `controllers/workspaceconfig/configmap2sync.go`

## Steps

1. Create a new `<type>2sync.go` file in `controllers/workspaceconfig/`
2. Implement the sync factory interface from `object2sync_factory.go`
3. Define how the source object maps to the target workspace namespace
4. The `WorkspaceConfig` controller handles the reconciliation loop — your factory just defines the transformation
5. Add tests following the pattern in `configmap2sync_test.go`
