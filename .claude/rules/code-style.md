# Code Style Rules

- Add an empty line after logical blocks of code (e.g., after `if` blocks, loops, variable declaration groups) to improve readability.
- Wrap errors with context using `fmt.Errorf` at every call site instead of passing them through directly — this applies both inside the function and at the caller. When the object has a namespace, include it as `namespace/name` in the message. Use `return fmt.Errorf("failed to sync deployment %s/%s: %w", obj.Namespace, obj.Name, err)` instead of `return err`.
- Lines should not exceed 120 characters. If a function call exceeds this limit, put each argument on its own line. This does not apply to `fmt.Errorf`, `fmt.Sprintf`, and similar formatting functions — keep those on one line.
- Use `deploy.GetLabels(component)` for labeling resources in `pkg/deploy/` reconcilers.
