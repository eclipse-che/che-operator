# 1. Pre-Requisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [Operator SDK v0.10.0](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)

WARNING: Please make sure to use the precise `v0.10.0` version of the `operator-sdk`. If you use a more recent version, you might generate a CRD that is not compatible with Kubernetes 1.11 and Openshift 3.11 (see issue https://github.com/eclipse/che/issues/15396).

If these dependencies are not installed, `docker-run.sh` can be used as a container bootstrap to run a given script with the appropriate dependencies.

Example : `$ docker-run.sh update-nightly-bundle.sh`

# 2. Eclipse Che Olm bundles

There two "nightly" platform specific Olm bundles:

`deploy/olm-catalog/eclipse-che-preview-kubernetes/manifests`
`deploy/olm-catalog/eclipse-che-preview-openshift/manifests`

Each bundle consists of a cluster service version file(CSV) and a custom resource definition file(CRD). 
CRD file describes "checluster" kubernetes api resource object(object fields name, format, description and so on).
Kubernetes api needs this information to correctly store a custom resource object "checluster".
Custom resource object users could modify to change Eclipse Che configuration.
Che operator watches "checluster" object and re-deploy Che with desired configuration.
The CSV file contains all "deploy" and "permission" specific information, which Olm needs to install The Eclipse Che operator.

# 3. Make new changes to OLM bundle

In `olm` folder

- If all dependencies are installed on the system:

```bash
$ ./update-nightly-bundle.sh
```

- To use a docker environment

```bash
$ ./docker-run.sh update-nightly-bundle.sh
```

Every change will be included to the deploy/olm-catalog bundles and override all previous changes.

To update a bundle without version incrementation and time update you can use env variables `NO_DATE_UPDATE` and `NO_INCREMENT`. For example, during development you need to update bundle a lot of times with changed che-operator deployment or role, rolebinding and etc, but you want to increment the bundle version and time creation, when all desired changes were completed:

```bash
$ export NO_DATE_UPDATE="true" && export NO_INCREMENT="true" && ./update-nightly-bundle.sh
```

# 4. Test scripts pre-requisites
Start your kubernetes/openshift cluster. For openshift cluster make sure that you was logged in like
"system:admin" or "kube:admin".

# 5.Test installation "stable" Eclipse Che using Application registry(Deprecated)
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

## 6. Test installation "nightly" Eclipse Che using CatalogSource(index) image

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

### 7. Build custom nightly bundle images

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

### 7.1 Testing custom CatalogSource and bundle images on the Openshift

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

### 7.2 Testing custom CatalogSource and bundle images on the Kubernetes
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

### 8. Test scripts arguments
There are some often used test script arguments:
 - `platform` - 'openshift' or 'kubernetes'
 - `channel` - installation Olm channel: 'nightly' or 'stable'
 - `namespace` - kubernetes namespace to deploy che-operator, for example 'che'
 - `optional-source-install` - installation method: 'Marketplace'(deprecated olm feature) or 'catalog'. By default will be used 'Marketplace'.

### 9. Debug test scripts
To debug test scripts you can use the "Bash debug" VSCode extension. 
For a lot of test scripts you can find different debug configurations in the `.vscode/launch.json`.
