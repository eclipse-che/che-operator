# Operator lifecycle manager

## Prerequisites

OLM packages scripts are using some required dependencies that need to be installed
 - [curl](https://curl.haxx.se/)
 - [https://github.com/kislyuk/yq](https://github.com/kislyuk/yq) and not [http://mikefarah.github.io/yq/](http://mikefarah.github.io/yq/)
 - [socat](http://www.dest-unreach.org/socat/)
 - [Operator SDK v0.17.2](https://github.com/operator-framework/operator-sdk/blob/v0.10.0/doc/user/install-operator-sdk.md)
 - [opm](https://github.com/operator-framework/operator-registry/releases/tag/v1.15.1)

WARNING: Please make sure to use the precise `v1.9.2` version of the `operator-sdk`.

## Eclipse Che OLM bundles

OLM (operator lifecycle manager) provides ways of installing operators. One of the convenient way how to achieve it is by using OLM bundles. See more about the format: https://github.com/openshift/enhancements/blob/master/enhancements/olm/operator-bundle.md. There two OLM bundles:

- `bundle/next/eclipse-che-preview-openshift/manifests` for the `next` channel
- `bundle/stable/eclipse-che-preview-openshift/manifests` for the `stable` channel

Each bundle consists of a cluster service version file (CSV) and a custom resource definition files (CRD). CRD file describes `checlusters` Kubernetes api resource object(object fields name, format, description and so on). Kubernetes api needs this information to correctly store a custom resource object "checluster". Custom resource object users could modify to change Eclipse Che configuration. Che operator watches `checlusters` object and re-deploy Che with desired configuration. The CSV file contains all "deploy" and "permission" specific information, which OLM needs to install Eclipse Che operator.

## Testing custom CatalogSource and next bundle images

To test next Che operator you have to use the OLM CatalogSource(index) image.
CatalogSource image stores in the internal database information about OLM bundles with different versions of the Eclipse Che.
Eclipse Che provides `quay.io/eclipse/eclipse-che-openshift-opm-catalog:next` catalog source images for the `next` channel.

For each new next version Eclipse Che provides the corresponding bundle image with name pattern:

`quay.io/eclipse/eclipse-che-openshift-opm-bundles:<CHE_VERSION>-<INCREMENTAL_VERSION>.next`

For example:

```
quay.io/eclipse/eclipse-che-openshift-opm-bundles:7.19.0-5.next
```

### Build custom next/stable OLM images

For test purpose you can build your own "next" or "stable" CatalogSource and bundle images
with your latest development changes and use it in the test scripts. To build these images you can use script `olm/buildCatalog.sh`:

```bash
$ olm/buildCatalog.sh \
    -c (next|stable) \
    -i <CATALOG_IMAGE>
```

### Testing custom CatalogSource and bundle images on the Openshift

To test the latest custom "next" bundle:

```bash
$ ./testCatalog.sh -c next -i <CATALOG_IMAGE> -n eclipse-che
```

If your CatalogSource image contains few bundles, you can test migration from previous bundle to the latest:

```bash
$ ./testUpdate.sh -c next -i <CATALOG_IMAGE> -n eclipse-che
```

# Install Eclipse Che from `stable` channel using testing catalog source image

Before publishing Eclipse Che in the community operator catalogs, we test new release using "stable" OLM channel
from testing catalog source image.

1. Create a custom catalog source:

```yaml
apiVersion:  operators.coreos.com/v1alpha1
kind:         CatalogSource
metadata:
  name:         eclipse-che-preview-custom
  namespace:    eclipse-che
spec:
  image:        quay.io/eclipse/eclipse-che-openshift-opm-catalog:test
  sourceType:  grpc
  updateStrategy:
    registryPoll:
      interval: 5m
```

2. Deploy Che operator:

```bash
$ chectl server:deploy --installer=olm --platform=openshift --catalog-source-yaml <PATH_TO_CUSTOM_CATALOG_SOURCE_YAML> --olm-channel=stable --package-manifest-name=eclipse-che-preview-openshift
```
