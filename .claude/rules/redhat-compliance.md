# Red Hat Compliance Rules

- If a code suggestion matches known open-source code, include the original license text and copyright notice. Do not suggest code whose license compatibility with EPL-2.0 cannot be verified.
- When suggesting commit messages, include `Assisted-by: {AGENT_NAME}` trailer (e.g., `Assisted-by: Claude Opus 4.6`).
- Never include credentials, tokens, or secrets in code.
- All new Go files must include the EPL-2.0 copyright header (copy from any existing file in the repo). This does not apply to files in the `vendor/` directory.
