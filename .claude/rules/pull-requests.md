# Pull Request Rules

- When creating a PR, use the project's PR template from `.github/PULL_REQUEST_TEMPLATE.md`.
- Fill in all sections: "What does this PR do?", "Screenshot/screencast of this PR", "What issues does this PR fix or reference?", and "How to test this PR?".
- Include deployment and test steps appropriate for the change (OpenShift via `test-catalog-from-sources.sh`, Minikube via `test-operator-from-sources.sh`, or both).
- Check all applicable items in the "Common Test Scenarios" checklist (deploy, start workspace, terminal, stop workspace, check operator logs).
- Check all applicable items in the "PR Checklist" (ECA, code complete, builds, tests, devfile, docs, CI/CD).
