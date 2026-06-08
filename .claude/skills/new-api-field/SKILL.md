---
name: new-api-field
description: How to add a new field to the CheCluster API
---

# Adding a New CheCluster API Field

Reference: existing fields in `api/v2/checluster_types.go`

## Steps

1. Add the field to the appropriate struct in `api/v2/checluster_types.go`
2. Add kubebuilder markers for validation, defaults, and CSV annotations
3. Run `make generate` to regenerate `zz_generated.deepcopy.go`
4. Run `make manifests` to regenerate CRD and RBAC manifests
5. Add v1↔v2 conversion logic (see `api-conversion` skill)
6. Add webhook validation if needed in `api/v2/checluster_webhook.go`
7. Run `make test` to verify
