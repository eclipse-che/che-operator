# Build & Test Commands

- Build: `make build`
- All unit tests: `make test`
- Single package: `MOCK_API=true go test -mod=vendor ./pkg/deploy/gateway/...`
- Single test: `MOCK_API=true go test -mod=vendor ./controllers/che/... -run TestSpecificName -v`
- Format: `make fmt`
- Vet: `make vet`
- Lint: `make lint`
- After making changes, always build and run the relevant tests before reporting the task as complete.
