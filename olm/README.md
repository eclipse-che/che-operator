# 1. Pre-Requisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [socat](http://www.dest-unreach.org/socat/)
 - [Operator SDK v0.17.2](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)
 - [opm](https://github.com/operator-framework/operator-registry/releases/tag/v1.15.1) 

WARNING: Please make sure to use the precise `v0.17.2` version of the `operator-sdk`.

If these dependencies are not installed, `docker-run.sh` can be used as a container bootstrap to run a given script with the appropriate dependencies.

Example : `$ docker-run.sh update-nightly-bundle.sh`

# 2. Eclipse Che operator

Eclipse Che application it's a kubernetes api controller to install Eclipse Che application and manage its
lifecycle. It does such kind of operations for Eclipse Che: installation, update, change configuration, uninstallation, display current status and some useful information like links to Che services.
Che operator implemented using [operator framework](https://github.com/operator-framework) and golang programming language. Eclipse Che configuration defined using custom resource definition object and stored in the custom kubernetes resouce named CheCluster(kubernetes api group `org.eclipse.checluster`). Che operator extends kubernetes api to embed Eclipse Che to Kubernetes cluster in a native way.

# 3. Che cluster custom resource definition

Che cluster custom resource definition(CRD) defines Eclipse CheCluster custom resource object. It contains information about object structure, field types, field descriptions.
CRD file - it's a yaml definition located in the `deploy/crds/org_v1_che_crd.yaml`.
But this file is auto-generated. So, to update this file you should not edit this file directly.
If you want to add new fields or fix descriptions in the crd you should make your
changes in the file `pkg/apis/org/v1/che_types.go` and launch script in the `olm` folder:

```bash
$ update-crd-files.sh
```

In the VSCode you can use task `Update cr/crd files`.

# 4. Che cluster custom resource
che-operator installs Eclipse Che using configuration stored in the kubernetes custom resource(CR).
CR object structure defined in the code using `pkg/apis/org/v1/che_types.go` file. Field name
defined using serialization tag `json`, for example `json:"openShiftoAuth"`.
Che operator default CR sample stored in the `deploy/crds/org_v1_che_cr.yaml`. 
This file should be directly modified if it's a required to apply new fields with default values,
or in case of changing default values for existed fields.
It is mandatory to update Olm bundle After modification CR sample to install che-operator using Olm.
Also user/developer could apply CR manually using `kubectl/oc apply -f deploy/crds/org_v1_che_cr.yaml -n namespace-che`.
But before that should be applied custom resource definion CRD, because kubernetes api need to get
information about new custom resource type and structure before storing custom resource.

# 5. Eclipse Che OLM bundles

OLM(operator lifycycle manager) provides a ways how to install operators. One of the convinient way how to
achieve it - it's a using OLM bundles. See more about format: https://github.com/openshift/enhancements/blob/master/enhancements/olm/operator-bundle.md
There two "nightly" platform specific OLM bundles for che-operator:

`deploy/olm-catalog/eclipse-che-preview-kubernetes/manifests`
`deploy/olm-catalog/eclipse-che-preview-openshift/manifests`

Each bundle consists of a cluster service version file(CSV) and a custom resource definition file(CRD). 
CRD file describes "checluster" kubernetes api resource object(object fields name, format, description and so on).
Kubernetes api needs this information to correctly store a custom resource object "checluster".
Custom resource object users could modify to change Eclipse Che configuration.
Che operator watches "checluster" object and re-deploy Che with desired configuration.
The CSV file contains all "deploy" and "permission" specific information, which Olm needs to install The Eclipse Che operator.

# 6. Make new changes to OLM bundle

Sometimes, during development, you need to modify some yaml definitions in the `deploy` folder 
or Che cluster custom resource. There are most frequently changes which should be included to the new Olm bundle:
  - operator deployment `deploy/operator.yaml`
  - operator role/cluster role permissions. They defined like role/rolebinding or cluster role/rolebinding yamls in the `deploy` folder.
  - operator custom resource CR `deploy/crds/org_v1_che_cr.yaml`. This file contains default CheCluster sample.
  Also this file is default Olm CheCluster sample.
  - Che cluster custom resource definition `deploy/crds/org_v1_che_cr.yaml`. For example you want
  to fix some propeties description or apply new che type properties with default values. 
  - add openshift ui annotations for che types properties to display information or interactive elements on the Openshift user interface.

For all these cases it's a nessuary to generate new Olm bundle to make these changes working with Olm.
So, first of all: make sure if you need to update crd, becasue crd it's a part of the Olm bundle.
See more about update [Che cluster CRD](Che_cluster_custom_resource_definition)

To generate new Olm bundle use script in `olm` folder

- If all dependencies are installed on the system:

```bash
$ ./update-nightly-bundle.sh
```

- To use a docker environment

```bash
$ ./docker-run.sh update-nightly-bundle.sh
```

Every change will be included to the deploy/olm-catalog bundles and override all previous changes.
Olm bundle changes should be commited to the pull request.

To update a bundle without version incrementation and time update you can use env variables `NO_DATE_UPDATE` and `NO_INCREMENT`. For example, during development you need to update bundle a lot of times with changed che-operator deployment or role, rolebinding and etc, but you want to increment the bundle version and time creation, when all desired changes were completed:

```bash
$ export NO_DATE_UPDATE="true" && export NO_INCREMENT="true" && ./update-nightly-bundle.sh
```

In the VSCode you can use task `Update csv bundle files`.

# 7. Test scripts pre-requisites
Start your kubernetes/openshift cluster. For openshift cluster make sure that you was logged in like
"system:admin" or "kube:admin".

# 8.Test installation "stable" Eclipse Che using Application registry(Deprecated)
Notice: this stuf doesn't for openshift >=4.6

To test stable versions che-operator you have to use Eclipse Che application registry.

To test the latest stable Che launch test script in the olm folder:

```bash
$ ./testCatalogSource.sh ${platform} "stable" ${namespace} "Marketplace"
```

To test migration from one stable version to another one:

```bash
$ ./testUpdate.sh ${platform} "stable" ${namespace}
```

See more information about test arguments in the chapter: [Test arguments](#test-script-arguments)

## 9. Test installation "nightly" Eclipse Che using CatalogSource(index) image

To test nightly che-operator you have to use Olm CatalogSource(index) image. 
CatalogSource image stores in the internal database information about Olm bundles with different versions of the Eclipse Che.
For nightly channel (dependent on platform) Eclipse Che provides two CatalogSource images:
 
 - `quay.io/eclipse/eclipse-che-kubernetes-opm-catalog:preview` for kubernetes platform;
 - `quay.io/eclipse/eclipse-che-openshift-opm-catalog:preview` for openshift platform;

For each new nightly version Eclipse Che provides nightly bundle image with name pattern:

`quay.io/eclipse/eclipse-che-${platform}-opm-bundles:${cheVersion}-${incrementVersion}.nightly`

For example:

```
quay.io/eclipse/eclipse-che-kubernetes-opm-bundles:7.18.0-1.nightly
...
quay.io/eclipse/eclipse-che-kubernetes-opm-bundles:7.19.0-5.nightly
...
```

To test the latest "nightly" bundle use `olm/testCatalogSource.sh` script:

```bash
$ ./testCatalogSource.sh ${platform} "nightly" ${namespace} "catalog"
```

To test migration Che from previous nightly version to the latest you can use `olm/testUpdate.sh` script:

```bash
$ ./testUpdate.sh ${platform} "nightly" ${namespace}
```

See more information about test arguments in the chapter: [Test arguments](#test-script-arguments)

### 10. Build custom nightly OLM images

For test purpose you can build your own "nightly" CatalogSource and bundle images
with your latest development changes and use it in the test scripts.
To build these images you can use script `olm/buildAndPushInitialBundle.sh`:

```bash
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
  ./buildAndPushInitialBundle.sh ${platform} ${optional-from-index-image}
```

This script will build and push for you two images: CatalogSource(index) image and bundle image:

```
"${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${PLATFORM}-opm-bundles:${cheVersion}-${incrementVersion}.nightly"
"${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/eclipse-che-${PLATFORM}-opm-catalog:preview"
```

CatalogSource images are additive. It's mean that you can re-use bundles from another CatalogSource image and
include them to your custom CatalogSource image. For this purpose you can specify the argument `optional-from-index-image`. For example:

```bash
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
  ./buildAndPushInitialBundle.sh "openshift" "quay.io/eclipse/eclipse-che-openshift-opm-catalog:preview"
```

### 11.1 Testing custom CatalogSource and bundle images on the Openshift

To test the latest custom "nightly" bundle use `olm/TestCatalogSource.sh`. For Openshift platform script build your test bundle: `deploy/olm-catalog/eclipse-che-preview-${platform}/manifests` using Openshift image stream:

```bash
$ ./testCatalogSource.sh "openshift" "nightly" ${namespace} "catalog"
```

If your CatalogSource image contains few bundles, you can test migration from previous bundle to the latest:

```bash
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
  ./testUpdate.sh "openshift" "nightly" ${namespace}
```

### 11.2 Testing custom CatalogSource and bundle images on the Kubernetes
To test your custom CatalogSource and bundle images on the Kubernetes you need to use public image registry.

For "docker.io" you don't need any extra steps with pre-creation image repositories. But for "quay.io" you should pre-create the bundle and and catalog image repositories manually and make them publicly visible. If you want to save repositories "private", then it is not necessary to pre-create them, but you need to provide an image pull secret to the cluster to prevent image pull 'unauthorized' error.

You can test your custom bundle and CatalogSource images:

```bash 
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
 ./testCatalogSource.sh "kubernetes" "nightly" ${namespace} "catalog"
```

If your CatalogSource image contains few bundles, you can test migration from previous bundle to the latest:

```bash
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
  ./testUpdate.sh "kubernetes" "nightly" ${namespace}
```

Also you can test your changes without a public registry. You can use the minikube cluster and enable the minikube "registry" addon. For this purpose we have script
`olm/minikube-private-registry.sh`. This script creates port forward to minikube private registry thought `localhost:5000`:

```bash
$ minikube-registry-addon.sh
```

This script should be launched before test execution in the separated terminal. To stop this script you can use `Ctrl+C`. You can check that private registry was forwarded to the localhost:

```bash
$ curl -X GET localhost:5000/v2/_catalog
{"repositories":[]}
```

With this private registry you can test installation Che from development bundle:

```bash
$ export IMAGE_REGISTRY_HOST="localhost:5000" && \
  export IMAGE_REGISTRY_USER_NAME="" && \
  ./testCatalogSource.sh kubernetes nightly che catalog
```

> Tips: If minikube was installed locally(driver 'none', local installation minikube), then registry is available on the host 0.0.0.0 without port forwarding.
But local installation minikube required 'sudo'.

### 12. Test script arguments
There are some often used test script arguments:
 - `platform` - 'openshift' or 'kubernetes'
 - `channel` - installation Olm channel: 'nightly' or 'stable'
 - `namespace` - kubernetes namespace to deploy che-operator, for example 'che'
 - `optional-source-install` - installation method: 'Marketplace'(deprecated olm feature) or 'catalog'. By default will be used 'Marketplace'.

### 13. che-operator CI pull request checks

che-operator uses two CI to launch all required checks for operator's pull requests:
  - github actions CI
  - openshift CI

Openshift CI configuration defined in the `https://github.com/openshift/release/tree/master/ci-operator/config/eclipse/che-operator` yamls. openshift ci scripts located in the che-operator repository in the folder `.ci`

Github actions defined in the `.github/workflows` yamls. Scripts located in the `.github/action_scripts`

To relaunch failed openshift CI checks onto your pull request you can use github comment `/retest`.
To relaunch failed github action checks you have to use github ui(`Re-run jobs` button). 
Unfortunately you can't relanch only one special check for github actions, github doesn't support such case.

### 14. Debug test scripts

To debug test scripts you can use the "Bash debug" VSCode extension. 
For a lot of test scripts you can find different debug configurations in the `.vscode/launch.json`.

### 15. Local debug che-operator(outside cluster)

To run che-operator in debug mode you can use `local-debug.sh` script. After process execution you should attach IDE debugger to debug port: 2345. Also you can see this port number in the process output.
`local-debug.sh` script has got two arguments:
 - `-n` - Eclipse Che installation namespace. Default value is `che`.
 - `-cr` - path to the custom resource yaml definition. Default value points to file `./deploy/crds/org_v1_che_cr.yaml`.
For VSCode we have launch.json configuration `Che Operator`. So in the VSCode debugger you can find this debug
configuration, select it, click `Start debugging` button. VSCode will debugger to che-operator process.

### 16. Check che-operator compilation

To check Che operator compilation you can use command:

```bash
GOOS=linux GOARCH=${ARCH} CGO_ENABLED=0 go build -mod=vendor -o /tmp/che-operator/che-operator cmd/manager/main.go
```

From command you can see operator will be compiled
to binary to folder `/tmp/che-operator/che-operator`. 
This command is usefull to make sure that che-opeator is still compiling after your changes.

Also we have corresponding VSCode task:`Check Che operator compilation`. 

### 18. Debug che-operator tests in VSCode 

To debug che-operator tests you can use VSCode `Launch Current File` configuration.
For that you have to open file with test, for example `pkg/controller/che/che_controller_test.go`,
set up some breakpoints, select `Launch Current File` configuration and click `Start debugging` button.

## 19. How to add new role/cluster role to che-operator

che-operator uses kubernetes api to manage Eclipse Che lifecycle.
To request any resouce objects from kubernetes api, che-operator should pass authorization.
Kubernetes api authorization uses role-based access control(RBAC) api(see more https://kubernetes.io/docs/reference/access-authn-authz/rbac/). che-operator has got service account, like process inside a pod.
Service account provides identity for che-operator. To service account can be bound some roles(or cluster roles).
This roles allows che-operator to request some kubernetes api resources.
Role(or cluster role) bindings we are using to bind roles(or cluster roles) to che-operator service account.

che-operator roles and cluster roles yaml definition located in the `deploy` folder.
For example `role.yaml`. When you need to provide access to new kubernetes api part for che-operator, you need
include to this file such information like:

  - api group
  - resource/s name
  - verbs - actions which che-operator can do on this resource/s

When this role/cluster role was updated you should also register `api` cshema in the `pkg/controller/che/che_controller.go` in the `add` method:

```go
import (
  ...
	userv1 "github.com/openshift/api/user/v1"
  ...
)

...
		if err := userv1.AddToScheme(mgr.GetScheme()); err != nil {
			logrus.Errorf("Failed to add OpenShift User to scheme: %s", err)
    }
...
```

Then you can use in che-operator controller code new kubernetes api part. When code complited and your pr is ready you should
generate new OLM bundle.

### 20. Using chectl to test che-operator

To test che-operator changes you can use chectl: https://github.com/che-incubator/chectl
chectl has got two installer types corresponding to che-operator: `operator` and `olm`.
With `operator` installer chectl reuses copies of che-operator deployment and role(cluster role) yamls from folder `deploy`.
With `olm` installer chectl uses catalog source index image with olm bundles from `deploy/olm-catalog`.
chectl supports cluster platforms: "minikube", "minishift", "k8s", "openshift", "microk8s", "docker-desktop", "crc".

### 20.1 Test che-operator with chectl and `operator` installer
If you want to test modified che-operator using chectl, you have to build custom che-operator image.

> Notice: you can use VSCode task `Build and push custom che-operator image`. But you need to specify
env variables: IMAGE_REGISTRY_HOST, IMAGE_REGISTRY_USER_NAME(for regular development you can use .bashrc for this purpose).
This task will build and push image with name `${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly`
to the registry.

Launch test cluster. Install Eclipse Che:

```bash
$ chectl deploy:server -n che --installer operator -p ${platform} --che-operator-image=${IMAGE_REGISTRY_HOST}/${IMAGE_REGISTRY_USER_NAME}/che-operator:nightly
```

where is `platform` it's a cluster platform.

> INFO: if you changed che-operator deployment or role/cluster role, CRD, CR you have to provide `--templates` argument.
This argument will points chectl to your modificated che-operator `deploy` folder path.

### 20.2 Test che-operator with chectl and `olm` installer
todo

### 20.3 Test che-operator with chectl and cr patch file
todo
