# Pre-Requisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [Operator SDK v0.10.0](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)

WARNING: Please make sure to use the precise `v0.10.0` version of the `operator-sdk`. If you use a more recent version, you might generate a CRD that is not compatible with Kubernetes 1.11 and Openshift 3.11 (see issue https://github.com/eclipse/che/issues/15396).

If these dependencies are not installed, `docker-run.sh` can be used as a container bootstrap to run a given script with the appropriate dependencies.

Example : `$ docker-run.sh update-nightly-olm-files.sh`


# Make new changes to OLM artifacts

Every change needs to be done in a new OLM artifact as previous artifacts are frozen.

A script is generating new folders/files that can be edited.

In `olm` folder

- If all dependencies are installed on the system:

```shell
$ update-nightly-olm-files.sh
```

- To use a docker environment

```shell
$ docker-run.sh update-nightly-olm-files.sh
```

Then the changes can be applied in the newly created CSV files.

## Local testing che-operator development version using OLM

To test che-operator with OLM you need to have application registry. You can register on the quay.io and
use application registry from this service.
Build your custom che-operator image and push it to the image registry(you also can use quay.io).
Change `deploy/operator.yaml` image from official to development.

Generate new nigthly olm bundle packages:

```shell
./update-nightly-olm-files.sh
```

Olm bundle packages will be generated in the folder `olm/eclipse-che-preview-${platform}`.

Push che-operator bundles to your application registry:

```shell
export QUAY_USERNAME=${username} && \
export QUAY_PASSWORD=${password} && \
export APPLICATION_REGISTRY=${application_registry_namespace} && \
./push-olm-files-to-quay.sh
```

Go to the quay.io and using ui make your application public.
Start minikube(or CRC) and after that lauch test script in the olm folder:

```shell
export APPLICATION_REGISTRY=${application_registry_namespace} &&  ./testCSV.sh ${platform} ${package_version} ${optional-namespace}
```

This script should install che-operator using OLM and check that Che server was deployed.
