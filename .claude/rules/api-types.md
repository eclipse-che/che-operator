---
paths:
  - "api/**"
---

# API Types Rules

- After modifying `api/v2/checluster_types.go`, always run `make generate` then `make manifests`.
- v2 is the storage version. v1 exists only for conversion compatibility.
- When adding a new field to v2, add corresponding conversion logic in `api/v1/checluster_conversion_to.go` and `api/v1/checluster_conversion_from.go`.
- Webhook validation and defaulting logic lives in `api/v2/checluster_webhook.go`.
- `zz_generated.deepcopy.go` files are generated — never edit them manually.
