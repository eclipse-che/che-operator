# `che-operator` release process

## 1. Release files

### Prerequisites
- export environment variables `QUAY_USERNAME` and `QUAY_PASSWORD`


```bash
./make-release.sh <RELEASE_VERSION>
```

## 2. Testing release on crc

Start a cluster using `cluster-bot` application.

To be able to test update it is needed to created some user before. Login as `kubeadmin`. Click `Update the cluster OAuth configuration` at the middle of the dashboard, then `Identity providers` -> `Add` -> `HTPassword` and upload a htpassword file (can be created with HTPassword utility). Logout and login using HTPassword, then logout and login as `kubeadmin`. Go to `kube:admin` -> `Copy Login Command` -> `Display Token` and launch showing command in the terminal. Now it is possible to test update:

```bash
olm/testUpdate.sh openshift stable
```

Open Eclipse Che dashboard in an anonymous tab:

```bash
echo http://$(oc get route -n eclipse-che-preview-test | grep ^che | awk -F ' ' '{ print $2 }')
```

Login using HTPassword then allow selected permissions. Validate that the release version is installed and workspace can be created:

## 3. Testing release on minikube

Run script to test updates:

```bash
olm/testUpdate.sh kubernetes stable
```

Open Eclipse Che dashboard:

```bash
xdg-open http://$(kubectl get ingress -n eclipse-che-preview-test | grep ^che | awk -F ' ' '{ print $2 }')
```

Validate that the release version is installed and workspace can be created:

## 4. Testing release on minishift

Login to local minishift cluster:

```bash
oc login <LOCAL_MINISHIFT_CLUSTER_ADDRESS>
```

Install the previous version of Eclipse Che:

```bash
chectl server:start --platform=minishift  --installer=operator --che-operator-image=quay.io/eclipse/che-operator:<PREVIOUS_RELEASE_VERSION>
```

Update Eclipse Che to the latest version. Validate that the correct version is installed and workspace can be created:

```bash
chectl server:update --platform=minishift  --installer=operator
xdg-open http://$(kubectl get ingress -n che | grep ^che | awk -F ' ' '{ print $2 }')
```

## 5. Prepare community operator PR

```bash
olm/prepare-community-operators-update.sh
```

TODO automate creating PRs
