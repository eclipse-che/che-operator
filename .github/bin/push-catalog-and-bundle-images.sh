#!/bin/bash
#
# Copyright (c) 2012-2020 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
set -ex

# install yq
pip3 install wheel
pip3 install --upgrade setuptools
pip3 install yq
# Make python3 installed modules "visible"
export PATH=$HOME/.local/bin:$PATH


export IMAGE_REGISTRY_USERNAME=eclipse
export IMAGE_REGISTRY=quay.io
export ROOT_PROJECT_DIR="${GITHUB_WORKSPACE}"
export BASE_DIR="${ROOT_PROJECT_DIR}/olm"

for platform in 'kubernetes' 'openshift'
do
  export OPM_BUNDLE_DIR="${ROOT_PROJECT_DIR}/deploy/olm-catalog/eclipse-che-preview-${platform}"
  export OPM_BUNDLE_MANIFESTS_DIR="${OPM_BUNDLE_DIR}/manifests"
  export CSV="${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

  export nightlyVersion=$(yq -r ".spec.version" "${CSV}")
  export CATALOG_BUNDLE_IMAGE_NAME_LOCAL="${IMAGE_REGISTRY}/${IMAGE_REGISTRY_USERNAME}/eclipse-che-${platform}-opm-bundles:${nightlyVersion}"
  export CATALOG_IMAGENAME="${IMAGE_REGISTRY}/${IMAGE_REGISTRY_USERNAME}/eclipse-che-${platform}-opm-catalog:preview"

  source "${ROOT_PROJECT_DIR}/olm/olm.sh" "${platform}" "${nightlyVersion}" "che"
  source "${ROOT_PROJECT_DIR}/olm/incrementNightlyBundles.sh"
  installOPM

  ${OPM_BINARY} version

  export incrementPart=$(getNightlyVersionIncrementPart "${nightlyVersion}")
  echo "[INFO] Nightly increment version ${incrementPart}"

  export CHECK_NIGHTLY_TAG=$(skopeo inspect docker://${IMAGE_REGISTRY}/${IMAGE_REGISTRY_USERNAME}/eclipse-che-${platform}-opm-bundles:${nightlyVersion} 2>/dev/null | jq -r ".RepoTags[]|select(. == \"${nightlyVersion}\")")
  if [ -z "$CHECK_NIGHTLY_TAG" ]
  then
    buildBundleImage "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"

    if [ "${incrementPart}" == 0 ]; then
      echo "[INFO] Build very first bundle."
      buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}"
    else
      buildCatalogImage "${CATALOG_IMAGENAME}" "${CATALOG_BUNDLE_IMAGE_NAME_LOCAL}" "docker" "${CATALOG_IMAGENAME}"
    fi

  else
      echo "[INFO] Bundle already present in the catalog source"
  fi
done
