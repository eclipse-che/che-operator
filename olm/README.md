# Pre-Requisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [Operator SDK v0.10.0](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)

WARNING: Please make sure to use the precise `v0.10.0` version of the `operator-sdk`. If you use a more recent version, you might generate a CRD that is not compatible with Kubernetes 1.11 and Openshift 3.11 (see issue https://github.com/eclipse/che/issues/15396).

If these dependencies are not installed, `docker-run.sh` can be used as a container bootstrap to run a given script with the appropriate dependencies.

Example : `$ docker-run.sh update-nightly-bundle.sh`


# Make new changes to OLM bundle

In `olm` folder

- If all dependencies are installed on the system:

```shell
$ ./update-nightly-bundle.sh
```

- To use a docker environment

```shell
$ ./docker-run.sh update-nightly-bundle.sh
```

Every change will be included to the deploy/olm-catalog/che-operator bundles and override all previous changes.

To update bundle without version incrementation and time update:

```shell
$ export NO_DATE_UPDATE="true" && export NO_INCREMENT="true" && export ./update-nightly-bundle.sh
```

## Local testing che-operator development version using OLM

To test a che-operator with OLM you need to have an application registry. You can register on the quay.io and
use application registry from this service.
Build your custom che-operator image and push it to the image registry(you also can use quay.io).
Change in the `deploy/operator.yaml` operator image from official to development.

Generate new nightly olm bundle packages:

```shell
$ ./update-nightly-bundle.sh
```

Olm bundle packages will be generated in the folders `deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}`.

Build custom olm bundle image with own nightly version:

```shell
$ 
```

Push image to the image registry, using docker or podman.

Build custom catalog image(index image) with created above bundle image. But there two options:
 - build catalog image with only one latest generated nightly version:

   ```shell
   $
   ```

 - build catalog image and include you latest generated nightly version **plus "all know nighlty versions from Eclipse Che nightly catalog source image**:

   ```shell
   $
   ```

Push images to the image registry.

## Push che-operator bundles to Application registry(Deprecated Olm feature)

Push che-operator bundles to your "quay" application registry:

```shell
$ export QUAY_ECLIPSE_CHE_USERNAME=${username} && \
  export QUAY_ECLIPSE_CHE_PASSWORD=${password} && \
  export APPLICATION_REGISTRY=${application_registry_namespace} && \
  ./push-olm-files-to-quay.sh
```

Go to the quay.io and use ui(tab Settings) to make your application public.

Start your kubernetes/openshift cluster. For openshift cluster make sure that you was logged in like
"system:admin" or "kube:admin". Launch test script in the olm folder:

```shell
$ export APPLICATION_REGISTRY=${application_registry_namespace} && \
  ./testCatalogSource.sh ${platform} ${channel} ${namespace} "Marketplace"
```

See more information about test arguments in the chapter: [Test arguments](#test-script-arguments)

> Notice: you can store security sensitive env variables in the `${HOME}/.bashrc`.

> Notice: if `APPLICATION_REGISTRY` was not defined, then tests script will use default Eclipse Che preview application registry. But it's make sence to use only with `stable` channel. `nightly` channel is not maintainable any more. To test nightly channel you should use catalog source(see next chapter).

## Test installation Eclipse Che using catalog source(index) image

To test che-operator with OLM files without push to a related Quay.io application, we can build a required docker olm bundle image and image with dedicated index image, in order to install directly through a CatalogSource.

Test script in the olm folder:

```shell
$ ./testCatalogSource.sh ${platform} ${channel} ${namespace} ${optional-source-install}
```

See more information about test arguments in the chapter: [Test arguments](#test-script-arguments)

> Warning: for "kubernetes" platform you need provide test images "storage". Test scripts supports two variants:
- specify env variables to provide access to the public image registry:
"IMAGE_REGISTRY_USER_NAME", "IMAGE_REGISTRY_PASSWORD", "IMAGE_REGISTRY_HOST". For image registry host "docker.io" it works without any extra actions:
```
$ export IMAGE_REGISTRY_USER_NAME=${userName} && \
  export IMAGE_REGISTRY_PASSWORD=${password} && \
  export IMAGE_REGISTRY_HOST=${imageRegistryHost} && \
 ./testCatalogSource.sh ${platform} ${channel} ${namespace} ${optional-source-install}
```
- use minikube cluster and enable minikube "registry" addon:
```shell
$ minikube addons enable registry
# Set up /etc/docker/daemon.json and docker service in most cases requires sudo.
$ mkdir -p "/etc/docker" && \
  touch "/etc/docker/daemon.json" && \
  config="{\"insecure-registries\" : [\"0.0.0.0:5000\"]}" && \
  echo "${config}" | sudo tee "${dockerDaemonConfig}" && \
  systemctl restart docker
```

This scripts should install che-operator using OLM and check that the Che server was deployed.

### Test migration Che from previous version to the latest
To test migration Che from previous version to the latest you can use `olm/testUpdate.sh` script:

```shell
$ ./testUpdate.sh ${platform} ${channel} ${namespace} ${optional-source-install}
```

See more information about test arguments in the chapter: [Test arguments](#test-script-arguments)

### Test script arguments
There are some often used test script arguments:
 - `platform` - 'openshift' or 'kubernetes'
 - `channel` - installation channel: 'nightly' or 'stable'
 - `namespace` - kubernetes namespace to deploy che-operator
 - `optional-source-install` - installation method: 'Marketplace'(deprecated olm feature) or 'catalog'. By default will be used 'Marketplace'.

### Debug test scripts
To debug tests scripts you can use "Bash debug" VSCode extension. 
For a lot of tests scripts you can find different debug configurations in the `.vscode/launch.json`.
