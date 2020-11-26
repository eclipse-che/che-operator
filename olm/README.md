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

Eclipse Che application is a Kubernetes API controller to install Eclipse Che application and manage its
lifecycle. It performs the following operations for Eclipse Che: installation, update, change configuration, uninstallation, display of the current status, and other useful information such as links to Che services.
Che operator is implemented using [operator framework](https://github.com/operator-framework) and the Go programming language. Eclipse Che configuration defined using a custom resource definition object and stored in the custom Kubernetes resource named CheCluster(Kubernetes API group `org.eclipse.checluster`). Che operator extends Kubernetes API to embed Eclipse Che to Kubernetes cluster in a native way.

# 3. Che cluster custom resource definition

Che cluster custom resource definition (CRD) defines Eclipse CheCluster custom resource object. It contains information about object structure, field types, field descriptions.
CRD file is a YAML definition located in the `deploy/crds/org_v1_che_crd.yaml`.
The file is auto-generated, so do not edit it directly to update it.
If you want to add new fields or fix descriptions in the CRD, make your
changes in the file `pkg/apis/org/v1/che_types.go` and launch the script in the `olm` folder:

```bash
$ ./update-crd-files.sh
```

> Notice: this script contains commands to make the CRD compatible with Openshift 3.

In the VSCode you can use the task `Update cr/crd files`.

# 4. Che cluster custom resource

che-operator installs Eclipse Che using configuration stored in the Kubernetes custom resource(CR).
CR object structure defined in the code using `pkg/apis/org/v1/che_types.go` file. Field name
defined using the serialization tag `json`, for example `json:"openShiftoAuth"`.
Che operator default CR sample is stored in the `deploy/crds/org_v1_che_cr.yaml`. 
This file should be directly modified if you want to apply new fields with default values,
or in case of changing default values for existing fields.
Also, you can apply in the field comments Openshift UI annotations: to display some
interactive information about these fields on the Openshift UI.
For example:

```go
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Eclipse Che URL"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:org.w3:link"
```

This comment-annotations displays clickable link on the Openshift ui with a text "Eclipse Che URL"

It is mandatory to update the OLM bundle after modification of the CR sample to deploy Eclipse Che using OLM.
Also users/developers could apply CR manually using `kubectl/oc apply -f deploy/crds/org_v1_che_cr.yaml -n ${che-namespace}`.
But before that should be applied custom resource definition CRD, because Kubernetes api need to get
information about new custom resource type and structure before storing the custom resource.

Openshift 4 provides UI to apply default CR. 
chectl includes CR automatically during deploy Che using 'operator' or 'olm' installer.

# 5. Eclipse Che OLM bundles

OLM (operator lifecycle manager) provides ways of installing operators. One of the convenient way how to
achieve it is by using OLM bundles. See more about the format: https://github.com/openshift/enhancements/blob/master/enhancements/olm/operator-bundle.md.
There two "nightly" platform-specific OLM bundles for che-operator:

- `deploy/olm-catalog/eclipse-che-preview-kubernetes/manifests`
- `deploy/olm-catalog/eclipse-che-preview-openshift/manifests`

Each bundle consists of a cluster service version file(CSV) and a custom resource definition file(CRD). 
CRD file describes "checluster" Kubernetes api resource object(object fields name, format, description and so on).
Kubernetes api needs this information to correctly store a custom resource object "checluster".
Custom resource object users could modify to change Eclipse Che configuration.
Che operator watches "checluster" object and re-deploy Che with desired configuration.
The CSV file contains all "deploy" and "permission" specific information, which OLM needs to install Eclipse Che operator.

# 6. Make new changes to OLM bundle

Sometimes, during development, you need to modify some YAML definitions in the `deploy` folder 
or Che cluster custom resource. There are most frequently changes which should be included to the new OLM bundle:
  - operator deployment `deploy/operator.yaml`
  - operator role/cluster role permissions. They are defined like role/rolebinding or cluster role/rolebinding yamls in the `deploy` folder.
  - operator custom resource CR `deploy/crds/org_v1_che_cr.yaml`. This file contains the default CheCluster sample.
  Also this file is the default OLM CheCluster sample.
  - Che cluster custom resource definition `pkg/apis/org/v1/che_types.go`.
  For example you want to fix some properties description 
  or apply new Che type properties with default values.
  These changes affect CRD `deploy/crds/org_v1_che_crd.yaml`.
  - add Openshift ui annotations for Che types properties(`pkg/apis/org/v1/che_types.go`) to display information or interactive elements on the Openshift user interface.

For all these cases it's a nessuary to generate a new OLM bundle to make these changes working with OLM.
So, first of all: make sure if you need to update CRD, because CRD it's a part of the OLM bundle.
See more about update [Che cluster CRD](Che_cluster_custom_resource_definition)

To generate new OLM bundle use script in `olm` folder

- If all dependencies are installed on the system:

```bash
$ ./update-nightly-bundle.sh
```

- To use a docker environment

```bash
$ ./docker-run.sh update-nightly-bundle.sh
```

Every change will be included to the `deploy/olm-catalog` bundles and override all previous changes.
OLM bundle changes should be committed to the pull request.

To update a bundle without version incrementation and time update you can use env variables `NO_DATE_UPDATE` and `NO_INCREMENT`. For example, during development you need to update bundle a lot of times with changed che-operator deployment or role, rolebinding and etc, but you want to increment the bundle version and time creation, when all desired changes were completed:

```bash
$ export NO_DATE_UPDATE="true" && export NO_INCREMENT="true" && ./update-nightly-bundle.sh
```

In the VSCode you can use task `Update csv bundle files`.

# 7. Test scripts pre-requisites

Start your Kubernetes/Openshift cluster. For Openshift cluster make sure that you was logged in like
"system:admin" or "kube:admin".

# 8.Test installation "stable" Eclipse Che using Application registry(Deprecated)

Notice: this stuff doesn't work for Openshift >= 4.6

To test stable versions che-operator you have to use Eclipse Che application registry.

To test the latest stable Che launch test script in the `olm` folder:

```bash
$ ./testCatalogSource.sh ${platform} "stable" ${namespace} "Marketplace"
```

To test migration from one stable version to another one:

```bash
$ ./testUpdate.sh ${platform} "stable" ${namespace}
```

See more information about test arguments in the chapter: [Test arguments](#test-script-arguments)

## 9. Test installation "nightly" Eclipse Che using CatalogSource(index) image

To test nightly che-operator you have to use the OLM CatalogSource(index) image. 
CatalogSource image stores in the internal database information about OLM bundles with different versions of the Eclipse Che.
For nightly channel (dependent on platform) Eclipse Che provides two CatalogSource images:
 
 - `quay.io/eclipse/eclipse-che-kubernetes-opm-catalog:preview` for Kubernetes platform;
 - `quay.io/eclipse/eclipse-che-openshift-opm-catalog:preview` for Openshift platform;

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
$ ./minikube-registry-addon.sh
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
 - `channel` - installation OLM channel: 'nightly' or 'stable'
 - `namespace` - Kubernetes namespace to deploy che-operator, for example 'che'
 - `optional-source-install` - installation method: 'Marketplace'(deprecated OLM feature) or 'catalog'. By default will be used 'Marketplace'.
