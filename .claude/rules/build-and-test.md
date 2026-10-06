# Build & Test Commands

- Build: `make build`
- Tests: `make test` — this is the only supported way to validate. It runs the whole suite and sets up
  the prerequisites (`download-gateway-resources`, `download-setup-envtest`, `KUBEBUILDER_ASSETS`).
- Do **not** validate with a bare `go test -mod=vendor ./package/...`. The envtest-based suites abort in
  `BeforeSuite` unless `KUBEBUILDER_ASSETS` points at the control-plane binaries, so a direct `go test`
  reports failures that are not real. If you need to narrow the run while iterating, export the assets
  first and treat it as a shortcut, not as validation:
  ```
  export KUBEBUILDER_ASSETS=$(bin/setup-envtest use 1.34.x --bin-dir "$(pwd)/bin/testbin" -p path)
  go test -mod=vendor ./package/... -run TestSpecificName -v
  ```
- Format: `make fmt`
- Vet: `make vet`
- Lint: `make lint`
- After making changes, always build and run `make test` before reporting the task as complete.
