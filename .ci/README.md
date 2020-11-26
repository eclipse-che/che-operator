### Che-Operator CI
Che Operator currently support two CI launch cases for Pull Request checks:
  - [Github Actions](https://github.com/eclipse/che-operator/actions)
  - [Openshift CI](https://prow.ci.openshift.org/?job=*che*operator*)

#### Openshift CI

Openshift is a Kubernetes based CI/CD system. Jobs can be triggered by various types of events and report their status to many different services. In addition to job execution, Openshift CI provides GitHub automation in the form of policy enforcement, chat-ops via /foo style commands, and automatic PR merging.

All documentation about how to onboard components in Openshift CI can be found in the Openshift CI jobs [repository](https://github.com/openshift/release). One of the requirements to make changes in Openshift CI jobs is being an openshift GitHub member.

All che-operator jobs configurations are defined in `https://github.com/openshift/release/tree/master/ci-operator/config/eclipse/che-operator`.

###### Jobs
- Eclipse Che Updates. This job basically installs the latest official Eclipse Che release and then update Che to the new release detected in PR. Note this PR check runs against `main` branch and all 7.* branches.
- Eclipse Che Nightly OLM files. A job that deploys Eclipse Che nightly using the latest version.

All Openshift CI checks name in pull request have a special nomenclature. Example: ci/prow/v3-che-operator-update where: `v3` is Openshift4 version and `che-operator-update` is the name of the job.

###### Triggers
All available plugins to trigger in GitHub Pull Request can be found [here](https://github.com/openshift/release/blob/master/core-services/prow/02_config/_plugins.yaml#L3607). The most important plugin is `test ?`, this trigger displays all the available triggers in the PR. 
In case of job failure openshift-robot write a comment about how to trigger a job which fail.

#### Github Actions

GitHub Actions is an API for cause and effect on GitHub: orchestrate any workflow, based on any event, while GitHub manages the execution, provides rich feedback, and secures every step along the way.

All che operator actions are defined in the `.github/workflows` yamls. Scripts are located in `.github/action_scripts` folder. 

###### Jobs
For minikube we currently have:
- Eclipse Che Updates. This job basically install last official Eclipse Che release and then update Che to the new release detected in PR. Note this PR check runs against `main` branch and all 7.* branches.
- Eclipse Che Nightly OLM files. Job which deploy Eclipse Che nightly using latest version.
- Eclipse Che Single host mode. Deploy eclipse che in single host mode(native and gateway) and verify the workspaces startups.

For Minishift currently we have:
- Eclipse Che Update
- Deploy Eclipse Che nightly

###### Triggers
To relaunch failed github action checks you have to use github ui(`Re-run jobs` button). Note the button execute the whole worflow.
