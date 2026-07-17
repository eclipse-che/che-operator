# Resource Naming Rules

- Resource names must not hardcode `eclipse-che`, `che`, or `devspaces`. Use `defaults.GetCheFlavor()` (from `pkg/common/operator-defaults`) to make names work for both upstream (`eclipse-che`/`che`) and downstream (`devspaces`).
- This applies to Kubernetes object names, label values, and any string embedded in resource specs that identifies the product. Constants like `constants.CheEclipseOrg` are fine — they are shared across both flavors.
