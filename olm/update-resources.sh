#!/bin/bash
#
# Copyright (c) 2019-2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation

set -e

unset UBI8_MINIMAL_IMAGE
unset PLUGIN_BROKER_METADATA_IMAGE
unset PLUGIN_BROKER_ARTIFACTS_IMAGE
unset JWT_PROXY_IMAGE

SCRIPT=$(readlink -f "${BASH_SOURCE[0]}")
ROOT_PROJECT_DIR=$(dirname $(dirname ${SCRIPT}))

checkOperatorSDKVersion() {
  if [ -z "${OPERATOR_SDK_BINARY}" ]; then
    OPERATOR_SDK_BINARY=$(command -v operator-sdk)
    if [[ ! -x "${OPERATOR_SDK_BINARY}" ]]; then
      echo "[ERROR] operator-sdk is not installed."
      exit 1
    fi
  fi

  local operatorVersion=$("${OPERATOR_SDK_BINARY}" version)
  REQUIRED_OPERATOR_SDK=$(yq -r ".\"operator-sdk\"" "${ROOT_PROJECT_DIR}/REQUIREMENTS")
  [[ $operatorVersion =~ .*${REQUIRED_OPERATOR_SDK}.* ]] || { echo "operator-sdk ${REQUIRED_OPERATOR_SDK} is required"; exit 1; }

  if [ -z "${GOROOT}" ]; then
    echo "[ERROR] set up '\$GOROOT' env variable to make operator-sdk working"
    exit 1
  fi
}

generateCRD() {
  version=$1
  "${OPERATOR_SDK_BINARY}" generate k8s
  "${OPERATOR_SDK_BINARY}" generate crds --crd-version $version

  ensureLicense ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml
  ensureLicense ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd.yaml
  ensureLicense ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd.yaml
  ensureLicense ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd.yaml

  if [[ $version == "v1" ]]; then
    mv ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml ${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd.yaml
    echo "[INFO] Generated CRD v1 ${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd.yaml"
    echo "[INFO] Generated CRD v1 ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
    echo "[INFO] Generated CRD v1 ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd.yaml"
    echo "[INFO] Generated CRD v1 ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd.yaml"
  elif [[ $version == "v1beta1" ]]; then
    mv ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusters_crd.yaml ${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd-v1beta1.yaml
    mv ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd.yaml ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd-v1beta1.yaml
    mv ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd.yaml ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd-v1beta1.yaml
    mv ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd.yaml ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd-v1beta1.yaml
    removeRequiredAttribute ${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd-v1beta1.yaml
    removeRequiredAttribute ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd-v1beta1.yaml
    removeRequiredAttribute ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd-v1beta1.yaml
    removeRequiredAttribute ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd-v1beta1.yaml
    echo "[INFO] Generated CRD v1beta1 ${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd-v1beta1.yaml"
    echo "[INFO] Generated CRD v1beta1 ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd-v1beta1.yaml"
    echo "[INFO] Generated CRD v1beta1 ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd-v1beta1.yaml"
    echo "[INFO] Generated CRD v1beta1 ${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd-v1beta1.yaml"
  fi

  # removes some left overs after operator-sdk
  rm ${ROOT_PROJECT_DIR}/deploy/crds/__crd.yaml
}

# Removes `required` attributes for fields to be compatible with OCP 3.11
removeRequiredAttribute() {
  REQUIRED=false
  while IFS= read -r line
  do
      if [[ $REQUIRED == true ]]; then
          if [[ $line == *"- "* ]]; then
              continue
          else
              REQUIRED=false
          fi
      fi

      if [[ $line == *"required:"* ]]; then
          REQUIRED=true
          continue
      fi

      echo  "$line" >> $1.tmp
  done < "$1"
  mv $1.tmp $1
}

detectImages() {
  ubiMinimal8Version=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.version')
  ubiMinimal8Release=$(skopeo inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.release')
  UBI8_MINIMAL_IMAGE="registry.access.redhat.com/ubi8-minimal:"$ubiMinimal8Version"-"$ubiMinimal8Release
  skopeo inspect docker://$UBI8_MINIMAL_IMAGE > /dev/null

  wget https://raw.githubusercontent.com/eclipse-che/che-server/main/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties
  PLUGIN_BROKER_METADATA_IMAGE=$(cat /tmp/che.properties| grep "che.workspace.plugin_broker.metadata.image" | cut -d = -f2)
  PLUGIN_BROKER_ARTIFACTS_IMAGE=$(cat /tmp/che.properties | grep "che.workspace.plugin_broker.artifacts.image" | cut -d = -f2)
  JWT_PROXY_IMAGE=$(cat /tmp/che.properties | grep "che.server.secure_exposer.jwtproxy.image" | cut -d = -f2)

  echo "[INFO] UBI base image               : $UBI8_MINIMAL_IMAGE"
  echo "[INFO] Plugin broker metadata image : $PLUGIN_BROKER_METADATA_IMAGE"
  echo "[INFO] Plugin broker artifacts image: $PLUGIN_BROKER_ARTIFACTS_IMAGE"
  echo "[INFO] Plugin broker jwt proxy image: $JWT_PROXY_IMAGE"
}

updateRoles() {
  echo "[INFO] Updating roles with DW and DWCO roles"

  CLUSTER_ROLES=(
    https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/openshift/objects/devworkspace-controller-view-workspaces.ClusterRole.yaml
    https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/openshift/objects/devworkspace-controller-edit-workspaces.ClusterRole.yaml
    https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/openshift/objects/devworkspace-controller-leader-election-role.Role.yaml
    https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/openshift/objects/devworkspace-controller-proxy-role.ClusterRole.yaml
    https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/openshift/objects/devworkspace-controller-role.ClusterRole.yaml
    https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/openshift/objects/devworkspace-controller-view-workspaces.ClusterRole.yaml
    https://raw.githubusercontent.com/che-incubator/devworkspace-che-operator/main/deploy/deployment/openshift/objects/devworkspace-che-role.ClusterRole.yaml
    https://raw.githubusercontent.com/che-incubator/devworkspace-che-operator/main/deploy/deployment/openshift/objects/devworkspace-che-metrics-reader.ClusterRole.yaml
  )

  # Updates cluster_role.yaml based on DW and DWCO roles
  ## Removes old cluster roles
  cat $ROOT_PROJECT_DIR/deploy/cluster_role.yaml | sed '/CHE-OPERATOR ROLES ONLY: END/q0' > $ROOT_PROJECT_DIR/deploy/cluster_role.yaml.tmp
  mv $ROOT_PROJECT_DIR/deploy/cluster_role.yaml.tmp $ROOT_PROJECT_DIR/deploy/cluster_role.yaml

  ## Copy new cluster roles
  for roles in "${CLUSTER_ROLES[@]}"; do
    echo "  # "$(basename $roles) >> $ROOT_PROJECT_DIR/deploy/cluster_role.yaml

    CONTENT=$(curl -sL $roles | sed '1,/rules:/d')
    while IFS= read -r line; do
      echo "  $line" >> $ROOT_PROJECT_DIR/deploy/cluster_role.yaml
    done <<< "$CONTENT"
  done

  ROLES=(
    https://raw.githubusercontent.com/che-incubator/devworkspace-che-operator/main/deploy/deployment/openshift/objects/devworkspace-che-leader-election-role.Role.yaml
  )

  # Updates role.yaml
  ## Removes old roles
  cat $ROOT_PROJECT_DIR/deploy/role.yaml | sed '/CHE-OPERATOR ROLES ONLY: END/q0' > $ROOT_PROJECT_DIR/deploy/role.yaml.tmp
  mv $ROOT_PROJECT_DIR/deploy/role.yaml.tmp $ROOT_PROJECT_DIR/deploy/role.yaml


  ## Copy new roles
  for roles in "${ROLES[@]}"; do
    echo "# "$(basename $roles) >> $ROOT_PROJECT_DIR/deploy/role.yaml

    CONTENT=$(curl -sL $roles | sed '1,/rules:/d')
    while IFS= read -r line; do
      echo "$line" >> $ROOT_PROJECT_DIR/deploy/role.yaml
    done <<< "$CONTENT"
  done
}

updateOperatorYaml() {
  OPERATOR_YAML="${ROOT_PROJECT_DIR}/deploy/operator.yaml"
  yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_pvc_jobs\") | .value ) = \"${UBI8_MINIMAL_IMAGE}\"" ${OPERATOR_YAML}
  yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_metadata\") | .value ) = \"${PLUGIN_BROKER_METADATA_IMAGE}\"" ${OPERATOR_YAML}
  yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_artifacts\") | .value ) = \"${PLUGIN_BROKER_ARTIFACTS_IMAGE}\"" ${OPERATOR_YAML}
  yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"${JWT_PROXY_IMAGE}\"" ${OPERATOR_YAML}

  # Deletes old DWCO container
  yq -riY "del(.spec.template.spec.containers[1])" $OPERATOR_YAML
  yq -riY ".spec.template.spec.containers[1] = \"devworkspace-container\"" $OPERATOR_YAML

  # Extract DWCO container spec from deployment
  DWCO_CONTAINER=$(curl -sL https://raw.githubusercontent.com/che-incubator/devworkspace-che-operator/main/deploy/deployment/openshift/objects/devworkspace-che-manager.Deployment.yaml \
    | sed '1,/containers:/d' \
    | sed -n '/serviceAccountName:/q;p' \
    | sed -e 's/^/  /')
  echo "$DWCO_CONTAINER" > dwcontainer

  # Add DWCO container to operator.yaml
  sed -i -e '/- devworkspace-container/{r dwcontainer' -e 'd}' $OPERATOR_YAML
  rm dwcontainer

  # update securityContext
  yq -riY ".spec.template.spec.containers[1].securityContext.privileged = false" ${OPERATOR_YAML}
  yq -riY ".spec.template.spec.containers[1].securityContext.readOnlyRootFilesystem = false" ${OPERATOR_YAML}
  yq -riY ".spec.template.spec.containers[1].securityContext.capabilities.drop[0] = \"ALL\"" ${OPERATOR_YAML}

  # update env variable
  yq -riY "del( .spec.template.spec.containers[1].env[] | select(.name == \"CONTROLLER_SERVICE_ACCOUNT_NAME\") | .valueFrom)" ${OPERATOR_YAML}
  yq -riY "( .spec.template.spec.containers[1].env[] | select(.name == \"CONTROLLER_SERVICE_ACCOUNT_NAME\") | .value) = \"che-operator\"" ${OPERATOR_YAML}
  yq -riY "del( .spec.template.spec.containers[1].env[] | select(.name == \"WATCH_NAMESPACE\") | .value)" ${OPERATOR_YAML}
  yq -riY "( .spec.template.spec.containers[1].env[] | select(.name == \"WATCH_NAMESPACE\") | .valueFrom.fieldRef.fieldPath) = \"metadata.namespace\"" ${OPERATOR_YAML}

  yq -riY ".spec.template.spec.containers[1].args[1] =  \"--metrics-addr\"" ${OPERATOR_YAML}
  yq -riY ".spec.template.spec.containers[1].args[2] =  \"0\"" ${OPERATOR_YAML}

  ensureLicense $OPERATOR_YAML
}

updateDockerfile() {
  DOCKERFILE="${ROOT_PROJECT_DIR}/Dockerfile"
  sed -i 's|registry.access.redhat.com/ubi8-minimal:[^\s]* |'${UBI8_MINIMAL_IMAGE}' |g' $DOCKERFILE
}

updateNighltyBundle() {
  source ${ROOT_PROJECT_DIR}/olm/olm.sh

  for platform in 'kubernetes' 'openshift'
  do
    if [ -z "${NO_INCREMENT}" ]; then
      source "${ROOT_PROJECT_DIR}/olm/incrementNightlyBundles.sh"
      incrementNightlyVersion "${platform}"
    fi

    echo "[INFO] Updating OperatorHub bundle for platform '${platform}'"

    NIGHTLY_BUNDLE_PATH=$(getBundlePath "${platform}" "nightly")
    NEW_CSV=${NIGHTLY_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml
    newNightlyBundleVersion=$(yq -r ".spec.version" "${NEW_CSV}")
    echo "[INFO] Creation new nightly bundle version: ${newNightlyBundleVersion}"

    generateFolder=${NIGHTLY_BUNDLE_PATH}/generated
    rm -rf "${generateFolder}"
    mkdir -p "${generateFolder}/crds"

    # copy roles
    "${NIGHTLY_BUNDLE_PATH}/build-roles.sh"

    # copy operator.yaml
    operatorYaml=$(yq -r ".\"operator-path\"" "${NIGHTLY_BUNDLE_PATH}/csv-config.yaml")
    cp -rf "${operatorYaml}" "${generateFolder}"

    # copy CR/CRD
    cp -f "${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_cr.yaml" "${generateFolder}/crds"
    cp -f "${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd.yaml" "${generateFolder}/crds"

    # generate a new CSV
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

    cp -f "${ROOT_PROJECT_DIR}/deploy/crds/org_v1_che_crd.yaml" "${NIGHTLY_BUNDLE_PATH}/manifests"
    cp -f "${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_chebackupserverconfigurations_crd.yaml" "${NIGHTLY_BUNDLE_PATH}/manifests/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
    cp -f "${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterbackups_crd.yaml" "${NIGHTLY_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterbackups_crd.yaml"
    cp -f "${ROOT_PROJECT_DIR}/deploy/crds/org.eclipse.che_checlusterrestores_crd.yaml" "${NIGHTLY_BUNDLE_PATH}/manifests/org.eclipse.che_checlusterrestores_crd.yaml"
    CRD="${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
    if [[ $platform == "openshift" ]]; then
      yq -riSY  '.spec.preserveUnknownFields = false' $CRD
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

    # set Pod Security Context Posture
    yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostIPC") = false' "${NEW_CSV}"
    yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostNetwork") = false' "${NEW_CSV}"
    yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostPID") = false' "${NEW_CSV}"
    if [ "${platform}" == "openshift" ]; then
      yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[0].securityContext."allowPrivilegeEscalation") = false' "${NEW_CSV}"
      yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[0].securityContext."runAsNonRoot") = true' "${NEW_CSV}"
      yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[1].securityContext."allowPrivilegeEscalation") = false' "${NEW_CSV}"
      yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[1].securityContext."runAsNonRoot") = true' "${NEW_CSV}"
    fi

    # Format code.
    yq -rY "." "${NEW_CSV}" > "${NEW_CSV}.old"
    mv "${NEW_CSV}.old" "${NEW_CSV}"

    ensureLicense "${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
    ensureLicense "${NIGHTLY_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"
  done
}

ensureLicense() {
  if [[ $(sed -n '/^#$/p;q' $1) != "#" ]]; then
echo -e "#
#  Copyright (c) 2019-2021 Red Hat, Inc.
#    This program and the accompanying materials are made
#    available under the terms of the Eclipse Public License 2.0
#    which is available at https://www.eclipse.org/legal/epl-2.0/
#
#  SPDX-License-Identifier: EPL-2.0
#
#  Contributors:
#    Red Hat, Inc. - initial API and implementation
$(cat $1)" > $1
fi
}

checkOperatorSDKVersion
detectImages

pushd "${ROOT_PROJECT_DIR}" || true

generateCRD "v1beta1"
generateCRD "v1"
updateRoles
updateOperatorYaml
updateDockerfile
updateNighltyBundle

popd || true
