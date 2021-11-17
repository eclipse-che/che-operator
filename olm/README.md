# Operator lifecycle manager

## Prerequisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [socat](http://www.dest-unreach.org/socat/)
 - [Operator SDK v0.17.2](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)
 - [opm](https://github.com/operator-framework/operator-registry/releases/tag/v1.15.1)

WARNING: Please make sure to use the precise `v1.7.1` version of the `operator-sdk`.

## Eclipse Che OLM bundles

OLM (operator lifecycle manager) provides ways of installing operators. One of the convenient way how to achieve it is by using OLM bundles. See more about the format: https://github.com/openshift/enhancements/blob/master/enhancements/olm/operator-bundle.md. There two "next" platform-specific OLM bundles for Сhe operator:

- `bundle/next/eclipse-che-preview-kubernetes/manifests`
- `bundle/next/eclipse-che-preview-openshift/manifests`

Each bundle consists of a cluster service version file (CSV) and a custom resource definition file (CRD). CRD file describes `checlusters` Kubernetes api resource object(object fields name, format, description and so on). Kubernetes api needs this information to correctly store a custom resource object "checluster". Custom resource object users could modify to change Eclipse Che configuration. Che operator watches `checlusters` object and re-deploy Che with desired configuration. The CSV file contains all "deploy" and "permission" specific information, which OLM needs to install Eclipse Che operator.

## Testing custom CatalogSource and next bundle images

To test next Che operator you have to use the OLM CatalogSource(index) image.
CatalogSource image stores in the internal database information about OLM bundles with different versions of the Eclipse Che. For next channel (dependent on platform) Eclipse Che provides two CatalogSource images:

 - `quay.io/eclipse/eclipse-che-kubernetes-opm-catalog:next` for Kubernetes platform;
 - `quay.io/eclipse/eclipse-che-openshift-opm-catalog:next` for Openshift platform;

For each new next version Eclipse Che provides next bundle image with name pattern:

`quay.io/eclipse/eclipse-che-<openshift|kubernetes>-opm-bundles:<CHE_VERSION>-<INCREMENTAL_VERSION>.next`

For example:

```
quay.io/eclipse/eclipse-che-kubernetes-opm-bundles:7.18.0-1.next
quay.io/eclipse/eclipse-che-openshift-opm-bundles:7.19.0-5.next
```

### Build custom next/stable OLM images

For test purpose you can build your own "next" or "stable" CatalogSource and bundle images
with your latest development changes and use it in the test scripts. To build these images you can use script `olm/buildCatalog.sh`:

```bash
$ olm/buildCatalog.sh \
    -c (next|next-all-namespaces|stable|tech-preview-all-namespaces) \
    -p (openshift|kubernetes) \
    -i <CATALOG_IMAGE>
```

### Testing custom CatalogSource and bundle images on the Openshift

To test the latest custom "next" bundle:

```bash
$ ./testCatalog.sh -p openshift -c next -i <CATALOG_IMAGE> -n eclipse-che
```

If your CatalogSource image contains few bundles, you can test migration from previous bundle to the latest:

```bash
$ ./testUpdate.sh -p openshift -c next -i <CATALOG_IMAGE> -n eclipse-che
```

### Testing custom CatalogSource and bundle images on the Kubernetes

To test your custom CatalogSource and bundle images on the Kubernetes you need to use public image registry. For "docker.io" you don't need any extra steps with pre-creation image repositories. But for "quay.io" you should pre-create the bundle and catalog image repositories manually and make them publicly visible. If you want to save repositories "private", then it is not necessary to pre-create them, but you need to provide an image pull secret to the cluster to prevent image pull 'unauthorized' error.

To test the latest custom "next" bundle:

```bash
$ ./testCatalog.sh -p kubernetes -c next -i <CATALOG_IMAGE> -n eclipse-che
```

If your CatalogSource image contains few bundles, you can test migration from previous bundle to the latest:

```bash
$ ./testUpdate.sh -p kubernetes -c next -i <CATALOG_IMAGE> -n eclipse-che
```

Also you can test your changes without a public registry. You can use the minikube cluster and enable the minikube "registry" addon. For this purpose use the script:

```bash
$ olm/minikube-registry-addon.sh
```

This script creates port forward to minikube private registry: `127.0.0.1:5000`. Should be launched before test execution in the separated terminal. To stop this script you can use `Ctrl+C`. You can check that private registry was forwarded to the localhost:

```bash
$ curl -X GET localhost:5000/v2/_catalog
{"repositories":[]}
```

With this private registry you can test Che operator from development bundle:

```bash
$ olm/buildCatalog.sh -p kubernetes -c next -i 127.0.0.1:5000/test/catalog:test
$ olm/testCatalog.sh -p kubernetes -c next -i 127.0.0.1:5000/test/catalog:test -n eclipse-che
```

> Tips: If minikube was installed locally (driver 'none', local installation minikube), then registry is available on the host 0.0.0.0 without port forwarding but it requires `sudo`.

# Install stable "preview" Eclipse Che using chectl

Before publishing Eclipse Che in the community operator catalogs, we are testing new release using "stable" OLM channel
from "preview" catalog source image.
Stable "preview" Eclipse Che can be installed via chectl.

1. Create a custom catalog source yaml and define platform(openshift or kubernetes):

```yaml
apiVersion:  operators.coreos.com/v1alpha1
kind:         CatalogSource
metadata:
  name:         eclipse-che-preview-custom
  namespace:    <che-namespace>
spec:
  image:        quay.io/eclipse/eclipse-che-<openshift|kubernetes>-opm-catalog:test
  sourceType:  grpc
  updateStrategy:
    registryPoll:
      interval: 5m
```

2. Deploy Che operator:

```bash
$ chectl server:deploy --installer=olm --platform=<CHECTL_SUPPORTED_PLATFORM> --catalog-source-yaml <PATH_TO_CUSTOM_CATALOG_SOURCE_YAML> --olm-channel=stable --package-manifest-name=eclipse-che-preview-<openshift|kubernetes>
```
