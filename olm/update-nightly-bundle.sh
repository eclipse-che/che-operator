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

set -e

if [ -z "${BASE_DIR}" ]; then
  BASE_DIR=$(cd "$(dirname "$0")"; pwd)
fi

if [ -z "${OPERATOR_SDK_BINARY}" ]; then
  OPERATOR_SDK_BINARY=$(command -v operator-sdk)
  if [[ ! -x "${OPERATOR_SDK_BINARY}" ]]; then
    echo "[ERROR] operator-sdk is not installed."
    exit 1
  fi
fi

# Check for compatible version of operator-sdk:
OPERATOR_SDK_VERSION=$(${OPERATOR_SDK_BINARY} version | cut -d, -f1 | cut -d: -f2 | sed 's/[ \"]//g')
REQUIRED_OPERATOR_SDK=$(yq -r ".\"operator-sdk\"" "${BASE_DIR}/../REQUIREMENTS")
case $OPERATOR_SDK_VERSION in
  "${REQUIRED_OPERATOR_SDK}")
    echo "Operator SDK ${OPERATOR_SDK_VERSION} installed"
    ;;
  *)
    echo "This script requires Operator SDK ${REQUIRED_OPERATOR_SDK}. Please install the correct version to continue"
    exit 1
    ;;
esac

OPERATOR_YAML="${BASE_DIR}"/../deploy/operator.yaml
NEW_OPERATOR_YAML="${OPERATOR_YAML}.new"

# copy licence header
eval head -10 "${OPERATOR_YAML}" > ${NEW_OPERATOR_YAML}

ROOT_PROJECT_DIR=$(dirname "${BASE_DIR}")
TAG=$1
source ${BASE_DIR}/check-yq.sh
source ${BASE_DIR}/olm.sh

ubiMinimal8Version=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.version')
ubiMinimal8Release=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.release')
UBI8_MINIMAL_IMAGE="registry.access.redhat.com/ubi8-minimal:"$ubiMinimal8Version"-"$ubiMinimal8Release
skopeo inspect docker://$UBI8_MINIMAL_IMAGE > /dev/null
wget https://raw.githubusercontent.com/eclipse/che/master/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties
PLUGIN_BROKER_METADATA_IMAGE_RELEASE=$(cat /tmp/che.properties| grep "che.workspace.plugin_broker.metadata.image" | cut -d = -f2)
PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE=$(cat /tmp/che.properties | grep "che.workspace.plugin_broker.artifacts.image" | cut -d = -f2)
JWT_PROXY_IMAGE_RELEASE=$(cat /tmp/che.properties | grep "che.server.secure_exposer.jwtproxy.image" | cut -d = -f2)

cat "${OPERATOR_YAML}" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_pvc_jobs\") | .value ) = \"${UBI8_MINIMAL_IMAGE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_metadata\") | .value ) = \"${PLUGIN_BROKER_METADATA_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_artifacts\") | .value ) = \"${PLUGIN_BROKER_ARTIFACTS_IMAGE_RELEASE}\"" | \
yq -ryY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"${JWT_PROXY_IMAGE_RELEASE}\"" \
>> "${NEW_OPERATOR_YAML}"
mv "${NEW_OPERATOR_YAML}" "${OPERATOR_YAML}"

DOCKERFILE=${BASE_DIR}/../Dockerfile
sed -i 's|registry.access.redhat.com/ubi8-minimal:.*|'${UBI8_MINIMAL_IMAGE}'|g' $DOCKERFILE

for platform in 'kubernetes' 'openshift'
do
  if [ -z "${NO_INCREMENT}" ]; then
    source "${BASE_DIR}/incrementNightlyBundles.sh"
    incrementNightlyVersion "${platform}"
  fi

  echo "[INFO] Updating OperatorHub bundle for platform '${platform}'"

  pushd "${ROOT_PROJECT_DIR}" || true

  NIGHTLY_BUNDLE_PATH=$(getBundlePath "${platform}" "nightly")
  bundleCSVName="che-operator.clusterserviceversion.yaml"
  NEW_CSV=${NIGHTLY_BUNDLE_PATH}/manifests/${bundleCSVName}
  newNightlyBundleVersion=$(yq -r ".spec.version" "${NEW_CSV}")
  echo "[INFO] Creation new nightly bundle version: ${newNightlyBundleVersion}"

  csv_config=${NIGHTLY_BUNDLE_PATH}/csv-config.yaml
  generateFolder=${NIGHTLY_BUNDLE_PATH}/generated
  rm -rf "${generateFolder}"
  mkdir -p "${generateFolder}"

  "${NIGHTLY_BUNDLE_PATH}/build-roles.sh"

  operatorYaml=$(yq -r ".\"operator-path\"" "${csv_config}")
  cp -rf "${operatorYaml}" "${generateFolder}/"

  crdsDir=${ROOT_PROJECT_DIR}/deploy/crds
  cp -rf "${crdsDir}" "${generateFolder}/"

  "${OPERATOR_SDK_BINARY}" generate csv \
  --csv-version "${newNightlyBundleVersion}" \
  --deploy-dir "${generateFolder}" \
  --output-dir "${NIGHTLY_BUNDLE_PATH}" 2>&1 | sed -e 's/^/      /'

  containerImage=$(sed -n 's|^ *image: *\([^ ]*/che-operator:[^ ]*\) *|\1|p' ${NEW_CSV})
  echo "[INFO] Updating new package version fields:"
  echo "[INFO]        - containerImage => ${containerImage}"
  sed -e "s|containerImage:.*$|containerImage: ${containerImage}|" "${NEW_CSV}" > "${NEW_CSV}.new"
  mv "${NEW_CSV}.new" "${NEW_CSV}"

  if [ -z "${NO_DATE_UPDATE}" ]; then
    createdAt=$(date -u +%FT%TZ)
    echo "[INFO]        - createdAt => ${createdAt}"
    sed -e "s/createdAt:.*$/createdAt: \"${createdAt}\"/" "${NEW_CSV}" > "${NEW_CSV}.new"
    mv "${NEW_CSV}.new" "${NEW_CSV}"
  fi

  if [ -z "${NO_INCREMENT}" ]; then
    incrementNightlyVersion "${platform}"
  fi

  templateCRD="${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd.yaml"
  platformCRD="${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"

  cp -rf $templateCRD $platformCRD
  if [[ $platform == "openshift" ]]; then
    yq -riSY  '.spec.preserveUnknownFields = false' $platformCRD
    yq -riSY  '.spec.validation.openAPIV3Schema.type = "object"' $platformCRD
    eval head -10 $templateCRD | cat - ${platformCRD} > tmp.crd && mv tmp.crd ${platformCRD}
  fi

  echo "Done for ${platform}"

  if [[ -n "$TAG" ]]; then
    echo "[INFO] Set tags in nightly OLM files"
    sed -ri "s/(.*:\s?)${RELEASE}([^-])?$/\1${TAG}\2/" "${NEW_CSV}"
  fi

  if [[ $platform == "openshift" ]]; then
    # Removes che-tls-secret-creator
    index=0
    while [[ $index -le 30 ]]
    do
      if [[ $(cat ${NEW_CSV} | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env['$index'].name') == "RELATED_IMAGE_che_tls_secrets_creation_job" ]]; then
        yq -rYSi 'del(.spec.install.spec.deployments[0].spec.template.spec.containers[0].env['$index'])' ${NEW_CSV}
        break
      fi
      index=$((index+1))
    done
  fi

  # Fix sample
  if [ "${platform}" == "openshift" ]; then
    echo "[INFO] Fix openshift sample"
    sample=$(yq -r ".metadata.annotations.\"alm-examples\"" "${NEW_CSV}")
    fixedSample=$(echo "${sample}" | yq -r ".[0] | del(.spec.k8s) | [.]" | sed -r 's/"/\\"/g')
    # Update sample in the CSV
    yq -rY " (.metadata.annotations.\"alm-examples\") = \"${fixedSample}\"" "${NEW_CSV}" > "${NEW_CSV}.old"
    mv "${NEW_CSV}.old" "${NEW_CSV}"
  fi
  if [ "${platform}" == "kubernetes" ]; then
    echo "[INFO] Fix kubernetes sample"
    sample=$(yq -r ".metadata.annotations.\"alm-examples\"" "${NEW_CSV}")
    fixedSample=$(echo "${sample}" | yq -r ".[0] | (.spec.k8s.ingressDomain) = \"\" | del(.spec.auth.openShiftoAuth) | [.]" | sed -r 's/"/\\"/g')
    # Update sample in the CSV
    yq -rY " (.metadata.annotations.\"alm-examples\") = \"${fixedSample}\"" "${NEW_CSV}" > "${NEW_CSV}.old"
    mv "${NEW_CSV}.old" "${NEW_CSV}"
  fi

  # set `app.kubernetes.io/managed-by` label
  yq -riSY  '(.spec.install.spec.deployments[0].spec.template.metadata.labels."app.kubernetes.io/managed-by") = "olm"' "${NEW_CSV}"

  # Format code.
  yq -rY "." "${NEW_CSV}" > "${NEW_CSV}.old"
  mv "${NEW_CSV}.old" "${NEW_CSV}"

  popd || true
done
