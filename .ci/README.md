# Che-Operator CI
Che Operator currently support two CI flows for Pull Request checks:
  - [Github Actions](https://github.com/eclipse-che/che-operator/actions)
  - [Openshift CI](https://prow.ci.openshift.org/?job=*che*operator*)

## Openshift CI

Openshift CI is a Kubernetes based CI/CD system. Jobs can be triggered by various types of events and report their status to many different services. In addition to job execution, Openshift CI provides GitHub automation in a form of policy enforcement, chat-ops via /foo style commands and automatic PR merging.

All documentation about how to onboard components in Openshift CI can be found in the Openshift CI jobs [repository](https://github.com/openshift/release). All Che operator jobs configurations are defined in `https://github.com/openshift/release/tree/master/ci-operator/config/eclipse-che/che-operator`.

- `operator-test` for testing Eclipse Che deployment
- `devworkspace-happy-path` for happy path testing (including workspace startup)
- `che-behind-proxy` for testing Eclipse Che deployment behind proxy
- `che-upgrade-stable-to-next` for testing Eclipse Che upgrade from the latest stable  to a new development version
- `che-operator-update` for testing Eclipse Che upgrade from the latest stable to a new release version

## Github Actions

All che operator actions are defined in the `.github/workflows` yamls. Scripts are located in `.github/bin/minikube` folder.

- `operator-on-minikube` for testing Eclipse Che deployment
- `backup-restore-test-on-minikub` for testing Eclipse Che backup and restore features
- `upgrade-stable-to-next-on-minikube` for testing Eclipse Che upgrade from the latest stable to a new development version
- `upgrade-stable-to-stable-on-minikube` for testing Eclipse Che upgrade from the latest stable to a new release version
