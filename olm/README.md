# Pre-Requisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [Operator SDK v0.10.0](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)

WARNING: Please make sure to use the precise `v0.10.0` version of the `operator-sdk`. If you use a more recent version, you might generate a CRD that is not compatible with Kubernetes 1.11 and Openshift 3.11 (see issue https://github.com/eclipse/che/issues/15396).

If these dependencies are not installed, `docker-run.sh` can be used as a container bootstrap to run a given script with the appropriate dependencies.

Example : `$ docker-run.sh update-nightly-olm-files.sh`


# Make new changes to OLM bundle

In `olm` folder

- If all dependencies are installed on the system:

```shell
$ update-nightly-olm-files.sh
```

- To use a docker environment

```shell
$ docker-run.sh update-nightly-olm-files.sh
```

Every change will be included to the deploy/olm-catalog/che-operator bundles and override all previous changes.

## Local testing che-operator development version using OLM

To test a che-operator with OLM you need to have an application registry. You can register on the quay.io and
use application registry from this service.
Build your custom che-operator image and push it to the image registry(you also can use quay.io).
Change in the `deploy/operator.yaml` operator image from official to development.

Generate new nightly olm bundle packages:

```shell
$ ./update-nightly-olm-files.sh
```

Olm bundle packages will be generated in the folders `deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}`.

Build custom olm bundle image with own nightly version:

```shell
$ 
```

Push image to the image registry, using dokcer or podman.

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

Push che-operator bundles to your application registry:

```shell
$ export QUAY_ECLIPSE_CHE_USERNAME=${username} && \
export QUAY_ECLIPSE_CHE_PASSWORD=${password} && \
export APPLICATION_REGISTRY=${application_registry_namespace} && \
./push-olm-files-to-quay.sh
```

Go to the quay.io and use ui(tab Settings) to make your application public.
Start minikube(or CRC) and after that launch test script in the olm folder:

```shell
$ export APPLICATION_REGISTRY=${application_registry_namespace} && ./testCSV.sh ${platform} ${package_version} ${optional-namespace}
```

Where are:
 - `platform` - 'openshift' or 'kubernetes'
 - `package_version` - your generated che-operator package version(for example: `7.8.0`)
 - `optional-namespace` - kubernetes namespace to deploy che-operator. Optional parameter, by default operator will be deployed to the namespace `eclipse-che-preview-test`

To test che-operator with OLM files without push to a related Quay.io application, we can build a required docker image of a dedicated catalog,
in order to install directly through a CatalogSource. To test this options start minikube and after that launch
test script in the olm folder:

```shell
export IMAGE_REGISTRY_USER_NAME=${username} && \
export IMAGE_REGISTRY_PASSWORD=${password} && \
export IMAGE_REGISTRY_HOST=${registry_name} \
$ ./testCatalogSource.sh {platform} ${channel} ${namespace}
```

This scripts should install che-operator using OLM and check that the Che server was deployed.
