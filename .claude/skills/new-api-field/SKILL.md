---
name: new-api-field
description: How to add a new field to the CheCluster API
---

# Adding a New CheCluster API Field

Reference: existing fields in `api/v2/checluster_types.go`

## Steps

1. Add the field to the appropriate struct in `api/v2/checluster_types.go`
2. Add kubebuilder markers for validation, defaults, and CSV annotations
3. Run `build/scripts/docker-run.sh make update-dev-resources` to regenerate CRDs, DeepCopy methods, and related manifests.
4. Add webhook validation if needed in `api/v2/checluster_webhook.go`
5. Run `make test` to verify
