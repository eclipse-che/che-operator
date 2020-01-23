# `che-operator` release process

## 1. Release files

```bash
./make-release.sh <RELEASE_VERSION>
```

## 2. Testing release on crc

Start a cluster using `cluster-bot` application.

To be able to test update it is needed to created some user before. Login as `kubeadmin`. At the middle of the dashboard click `Update OAuth configuration`. Then `Identidy providers` -> `Add` -> `HTPassword` and upload a htpassword file (can be created with HTPassword utility). Login using HTPassword, logout and login again as `kubeadmin`. Go to `kube:admin` menu -> `Copy Login Command` -> `Display Token` and launch to showing command in the terminal. Now it is possible to test update by launching:

```bash
olm/testUpdates.sh openshift stable
```

Open Eclipse Che dashboard to validate that the correct version is installed and workspace can be created:

```bash
echo http://$(kubectl get ingress -n che | grep ^che | awk -F ' ' '{ print $2 }')
```

Open the url in the anonymous tab.

## 3. Testing release on minikube

Run script to test updates:

```bash
olm/testUpdates.sh kubernetes stable
```

Open Eclipse Che dashboard to validate that the correct version is installed and workspace can be created:

```bash
xdg-open http://$(kubectl get ingress -n che | grep ^che | awk -F ' ' '{ print $2 }')
```

## 4. Testing release on minishift

Login to local minishift cluster:

```bash
oc login <LOCAL_MINISHIFT_CLUSTER_ADDRESS>
```

Install the previous version of Eclipse Che:

```bash
chectl server:start --platform=minikube  --installer=operator --che-operator-image=quay.io/eclipse/che-operator:<PREVIOUS_RELEASE_VERSION>
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
