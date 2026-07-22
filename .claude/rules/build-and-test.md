# Build & Test Commands

- Build: `make build`
- All unit tests: `make test`
- Single package: `MOCK_API=true go test -mod=vendor ./package/...`
- Single test: `MOCK_API=true go test -mod=vendor ./package/... -run TestSpecificName -v`
- Format: `make fmt`
- Vet: `make vet`
- Lint: `make lint`
- After making changes, always build and run the relevant tests before reporting the task as complete.
