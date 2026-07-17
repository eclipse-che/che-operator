# Kubebuilder Default Propagation Rule

- When adding a `+kubebuilder:default` annotation to a field, propagate the default up to every parent struct's `+kubebuilder:default` annotation as well.
- If the parent struct already has a `+kubebuilder:default`, add the new nested default to it. If it doesn't, add one.
- This is required because kubebuilder only applies a field's default when its parent object is non-nil. A pointer field (`*Foo`) with a default on its inner fields has no effect if the parent's default doesn't instantiate it.
- After changing any `+kubebuilder:default` annotation, run `make update-dev-resources` to regenerate the CRDs.
