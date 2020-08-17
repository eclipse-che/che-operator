# `che-operator` release process

## 1. Release files

Export environment variables:
1. `QUAY_ECLIPSE_CHE_USERNAME` and `QUAY_ECLIPSE_CHE_PASSWORD` to access https://quay.io/organization/eclipse

```bash
./make-release.sh <RELEASE_VERSION> --release --push-olm-files --push-git-changes --pull-requests
```

```
Usage:   ./make-release.sh [RELEASE_VERSION] --release --release-olm-files --push-olm-files --push-git-changes --pull-requests
        --release: to release
        --update-nightly-olm-files: generate new olm files for nightly version
        --release-olm-files: to release olm files
        --push-olm-files: to push OLM files to quay.io. This flag should be omitted
                if already a greater version released. For instance, we are releasing 7.9.3 version but
                7.10.0 already exists. Otherwise it breaks the linear update path of the stable channel.
        --push-git-changes: to create release branch and push changes into.
        --pull-requests: to create pull requests.
```

## 2. Testing release on openshift

Start a cluster using `cluster-bot` application.

To be able to test update it is needed to created some user before. Login as `kubeadmin`. Click `Update the cluster OAuth configuration` at the middle of the dashboard, then `Identity providers` -> `Add` -> `HTPassword` and upload a htpassword file (can be created with HTPassword utility). Logout and login using HTPassword, then logout and login as `kubeadmin`. Go to `kube:admin` -> `Copy Login Command` -> `Display Token` and launch showing command in the terminal. Now it is possible to test update:

```bash
cd olm
./testUpdate.sh openshift stable
```

Open Eclipse Che dashboard in an anonymous tab:

```bash
echo http://$(oc get route -n eclipse-che-preview-test | grep ^che | awk -F ' ' '{ print $2 }')
```

Login using HTPassword then allow selected permissions. Validate that the release version is installed and workspace can be created:

## 3. Testing release on minikube

Run script to test updates:

```bash
cd olm
./testUpdate.sh kubernetes stable
```

Open Eclipse Che dashboard:

```bash
xdg-open http://$(kubectl get ingress -n eclipse-che-preview-test | grep ^che | awk -F ' ' '{ print $2 }')
```

Validate that the release version is installed and workspace can be created:

## 4. Merge pull requests

Merge pull request into .x and master branches.

## 5. Testing release on minishift (when chectl is released)

Login to local minishift cluster:

```bash
oc login <LOCAL_MINISHIFT_CLUSTER_ADDRESS>
```

Install the previous version of Eclipse Che using the corresponding version of `chectl`:

```bash
chectl server:start --platform=minishift  --installer=operator
```

Update Eclipse Che to the latest version. Validate that the correct version is installed and workspace can be created:

```bash
chectl update stable
chectl server:update --platform=minishift  --installer=operator
xdg-open http://$(kubectl get ingress -n che | grep ^che | awk -F ' ' '{ print $2 }')
```

## 6. Prepare community operator PR

```bash
cd olm
./prepare-community-operators-update.sh
```
