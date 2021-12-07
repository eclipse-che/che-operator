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
Usage:   ./make-release.sh [RELEASE_VERSION] --release --release-olm-files --push-olm-bundles --push-git-changes --pull-requests
        --release: to release
        --release-olm-files: to release olm files
        --push-olm-bundles: to push OLM bundle images to quay.io. This flag should be omitted
                if already a greater version released. For instance, we are releasing 7.9.3 version but
                7.10.0 already exists. Otherwise it breaks the linear update path of the stable channel.
        --push-git-changes: to create release branch and push changes into.
        --pull-requests: to create pull requests.
```

## 2. Testing release on openshift

This part now runs automatically as part of the PR check for release PRs. See PROW CI checks in release PRs.
Alternatively, use these manual steps to verify operator update on Openshift.

Start a cluster using `cluster-bot` application.

To be able to test update it is needed to created some user before. Login as `kubeadmin`. Click `Update the cluster OAuth configuration` at the middle of the dashboard, then `Identity providers` -> `Add` -> `HTPassword` and upload a htpassword file (can be created with HTPassword utility). Logout and login using HTPassword, then logout and login as `kubeadmin`. Go to `kube:admin` -> `Copy Login Command` -> `Display Token` and launch showing command in the terminal. Now it is possible to test update:

```bash
cd olm
./testUpdate.sh -p openshift -c stable -i quay.io/eclipse/eclipse-che-openshift-opm-catalog:test -n eclipse-che
```

Open Eclipse Che dashboard in an anonymous tab:

```bash
echo http://$(oc get route -n eclipse-che-preview-test | grep ^che | awk -F ' ' '{ print $2 }')
```

Login using HTPassword then allow selected permissions. Validate that the release version is installed and workspace can be created:

## 3. Merge pull requests

Merge pull request into .x and main branches.

## 4. Testing release on minishift (when chectl is released)

Login to local minishift cluster:

```bash
oc login <LOCAL_MINISHIFT_CLUSTER_ADDRESS>
```

Install the previous version of Eclipse Che using the corresponding version of `chectl`:

```bash
chectl server:deploy --platform=minishift  --installer=operator
```

Update Eclipse Che to the latest version. Validate that the correct version is installed and workspace can be created:

```bash
chectl update stable
chectl server:update --platform=minishift  --installer=operator
xdg-open http://$(kubectl get ingress -n che | grep ^che | awk -F ' ' '{ print $2 }')
```

## 5. Prepare community operator PR

See `release-community-operator-PRs.yml` workflow, which will be triggered automatically, once release PRs are merged.
Alternatively, it can be run manually:

```bash
cd olm
./prepare-community-operators-update.sh
```
