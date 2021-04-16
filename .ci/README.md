# Che-Operator CI
Che Operator currently support two CI flows for Pull Request checks:
  - [Github Actions](https://github.com/eclipse-che/che-operator/actions)
  - [Openshift CI](https://prow.ci.openshift.org/?job=*che*operator*)

## Openshift CI

Openshift is a Kubernetes based CI/CD system. Jobs can be triggered by various types of events and report their status to many different services. In addition to job execution, Openshift CI provides GitHub automation in the form of policy enforcement, chat-ops via /foo style commands, and automatic PR merging.

All documentation about how to onboard components in Openshift CI can be found in the Openshift CI jobs [repository](https://github.com/openshift/release). One of the requirements to make changes in Openshift CI jobs is being an openshift GitHub member.

All Che operator jobs configurations are defined in `https://github.com/openshift/release/tree/master/ci-operator/config/eclipse-che/che-operator`.

### Jobs
- `che-operator-update` - It tests Eclipse Che update from the latest Eclipse Che release version to a new version from the PR and workspace startup. Note this PR check runs against `main` branch and all `7.x.y` branches.
- `che-operator-olm-latest-changes-tests`- It tests Eclipse Che deployment and workspace startup.

All Openshift CI checks name in pull request have a special nomenclature. Example: `ci/prow/v6-che-operator-update where` where `v6` is Openshift4 minor version.

### Triggers
All available plugins to trigger in GitHub Pull Request can be found [here](https://github.com/openshift/release/blob/master/core-services/prow/02_config/_plugins.yaml#L3607). The most important plugin is `test ?`, this trigger displays all the available triggers in the PR. In case of a job failure openshift-robot writes a comment about how to trigger a job which fails.

## Github Actions

GitHub Actions is an API for cause and effect on GitHub: orchestrate any workflow, based on any event, while GitHub manages the execution, provides rich feedback, and secures every step along the way.

All che operator actions are defined in the `.github/workflows` yamls. Scripts are located in `.github/action_scripts` folder.

### Actions

#### Minikube
For minikube we currently have:
- `Testing stable versions updates` - It tests Eclipse Che update from the latest Eclipse Che release version to a new version from the PR and workspace startup.
- `Testing latest changes`- It tests Eclipse Che deployment in multi host mode and workspace startup.
- `Testing latest changes (single-host/native)` - It tests Eclipse Che deployment in single host mode with `native` exposure type and workspace startup.
- `Testing latest changes (single-host/gateway)` - It tests Eclipse Che deployment in single host mode with `gateway` exposure type and workspace startup.

#### Minishift
- `Testing stable versions updates` - It tests Eclipse Che update from the latest Eclipse Che release version to a new version from the PR and workspace startup.
- `Testing latest changes` - It tests Eclipse Che deployment in multi host mode and workspace startup.
- `e2e tests` - It runs basic e2e tests.

### Triggers
To relaunch failed GitHub action checks you have to use GitHub UI (`Re-run jobs` button). Note the button executes the whole workflow.
