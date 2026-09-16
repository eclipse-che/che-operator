<!-- Please review the following before submitting a PR:
che-operator Development Guide: https://github.com/eclipse-che/che-operator/#development
-->

### What does this PR do?


### Screenshot/screencast of this PR
<!-- Please include a screenshot or a screencast explaining what is doing this PR -->


### What issues does this PR fix or reference?
<!-- Please include any related issue from eclipse che repository (or from another issue tracker).
     Include link to other pull requests like documentation PR from [the docs repo](https://github.com/eclipse/che-docs)
-->


### How to test this PR?
<!-- Please explain for example :
  - The test platform (openshift, kubernetes, minikube, docker-desktop, etc)
  - steps to reproduce
 -->

1. Deploy the operator:
 
#### OpenShift
```bash
oc create namespace devworkspace-controller
oc apply --server-side -f https://raw.githubusercontent.com/devfile/devworkspace-operator/refs/heads/main/deploy/deployment/openshift/combined.yaml

OPERATOR_IMAGE=<...>
sed 's|quay.io/eclipse/che-operator:next|'${OPERATOR_IMAGE}'|g' deploy/deployment/openshift/combined.yaml | oc apply --server-side -f  -
oc wait --for=condition=Ready pod -l app.kubernetes.io/component=che-operator -n eclipse-che --timeout=60s
oc apply --server-side -f deploy/deployment/openshift/org_v2_checluster.yaml
oc wait checluster eclipse-che -n eclipse-che --for=jsonpath='.status.chePhase'=Active   --timeout=120s
```

#### on Minikube

```bash
./build/scripts/minikube-tests/test-operator-from-sources.sh
```

#### Common Test Scenarios
- [ ] Deploy Eclipse Che
- [ ] Start an empty workspace
- [ ] Open terminal and build/run an image
- [ ] Stop a workspace
- [ ] Check operator logs for reconciliation errors or infinite reconciliation loops

### PR Checklist

[As the author of this Pull Request I made sure that:](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#pull-request-template-and-its-checklist)

- [ ] [The Eclipse Contributor Agreement is valid](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#the-eclipse-contributor-agreement-is-valid)
- [ ] [Code produced is complete](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#code-produced-is-complete)
- [ ] [Code builds without errors](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#code-builds-without-errors)
- [ ] [Tests are covering the bugfix](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#tests-are-covering-the-bugfix)
- [ ] [The repository devfile is up to date and works](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#the-repository-devfile-is-up-to-date-and-works)
- [ ] [Relevant user documentation updated](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#relevant-contributing-documentation-updated)
- [ ] [Relevant contributing documentation updated](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#relevant-contributing-documentation-updated)
- [ ] [CI/CD changes implemented, documented and communicated](https://github.com/eclipse/che/blob/master/CONTRIBUTING.md#cicd-changes-implemented-documented-and-communicated)

### Reviewers

Reviewers, please comment how you tested the PR when approving it.
