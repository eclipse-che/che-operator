---
name: api-conversion
description: How to handle v1 to v2 CheCluster API conversion
---

# API Conversion (v1↔v2)

Reference: existing fields in `api/v1/checluster_conversion_to.go` and `api/v1/checluster_conversion_from.go`

## Steps

1. Add v2→v1 mapping in `api/v1/checluster_conversion_from.go` (`convertFrom` methods)
2. Add v1→v2 mapping in `api/v1/checluster_conversion_to.go` (`convertTo` methods)
3. Add round-trip test cases in `api/checluster_round_conversion_test.go`
4. Run `make test` — conversion tests verify lossless round-tripping
