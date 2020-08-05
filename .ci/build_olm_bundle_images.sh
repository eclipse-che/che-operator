#!/bin/bash

DOCKER_USERNAME=aandriienko
IMAGE_REGISTRY=quay.io
ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
export BASE_DIR="${ROOT_PROJECT_DIR}/olm"

# install yq
pip3 install wheel
pip3 install --upgrade setuptools
pip3 install yq
# Make python3 installed modules "visible"
export PATH=$HOME/.local/bin:$PATH
ls $HOME/.local/bin

for platform in 'kubernetes' 'openshift'
do
  OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/che-operator/eclipse-che-preview-${platform}"
  OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
  CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

  nightlyVersion=$(yq -r ".spec.version" "${CSV}")
  CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${IMAGE_REGISTRY}/${DOCKER_USERNAME}/eclipse-che-${platform}-opm-bundles:${nightlyVersion}"
  CATALOG_IMAGENAME="${IMAGE_REGISTRY}/${DOCKER_USERNAME}/eclipse-che-${platform}-opm-catalog"

  # todo remove namespace
  source ${ROOT_PROJECT_DIR}/olm/olm.sh "${platform}" "${nightlyVersion}" "che"
  source ${ROOT_PROJECT_DIR}/olm/incrementNightlyBundles.sh

  installOPM

  ${OPM_BINARY} version

  incrementPart=$(getNightlyVersionIncrementPart "${nightlyVersion}")
  echo "Nightly increment version ${incrementPart}"

  buildBundleImage ${OPM_BUNDLE_MANIFESTS_DIR} "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

  if [ ${incrementPart} == 0 ]; then
    echo "Build very first bundle."
    buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"
  else
    buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" "${CATALOG_IMAGENAME}"
  fi
done
