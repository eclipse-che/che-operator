# `che-operator` release process

## 1. Release files

See `release.yml` workflow, which can be used to perform this step using GitHub Actions CI.
Alternatively, use these manual steps to prepare release pull requrests:

Export environment variables:
1. `QUAY_ECLIPSE_CHE_USERNAME` and `QUAY_ECLIPSE_CHE_PASSWORD` to access https://quay.io/organization/eclipse

```bash
./make-release.sh <RELEASE_VERSION> --release --push-olm-bundles --push-git-changes --pull-requests
```

```
Usage:   build/scripts/release/make-release.sh [RELEASE_VERSION] --release --release-olm-files --push-olm-bundles --push-git-changes --pull-requests
        --release: to release operator code
        --release-olm-files: to release olm files
        --push-olm-bundles: to push OLM bundle images to quay.io. This flag should be omitted
                if already a greater version released. For instance, we are releasing 7.9.3 version but
                7.10.0 already exists. Otherwise it breaks the linear update path of the stable channel.
        --push-git-changes: to create release branch and push changes into.
        --pull-requests: to create pull requests.
```

## 2. Testing release

This part now runs automatically as part of the PR check for release PRs. See PROW CI checks in release PRs.
Alternatively, use these manual steps to verify operator update on Openshift.

Start a cluster using `cluster-bot` application.

To be able to test update it is needed to create some user before. Login as `kubeadmin`. Click `Update the cluster OAuth configuration` at the middle of the dashboard, then `Identity providers` -> `Add` -> `HTPassword` and upload a htpassword file (can be created with HTPassword utility). Logout and login using HTPassword, then logout and login as `kubeadmin`. Go to `kube:admin` -> `Copy Login Command` -> `Display Token` and launch showing command in the terminal. Now it is possible to test update:

```bash
build/scripts/olm/testUpdate.sh -c stable -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -n eclipse-che
```

## 3. Merge pull requests

Merge pull request into .x and main branches.

## 4. Prepare community operator PR

See `release-community-operator-PRs.yml` workflow, which will be triggered automatically, once release PRs are merged.
Alternatively, it can be run manually:

```bash
./prepare-community-operators-update.sh
```
