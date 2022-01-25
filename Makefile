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
#

# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 1.0.2


ifndef VERBOSE
MAKEFLAGS += --silent
endif

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir := $(dir $(mkfile_path))

OPERATOR_SDK_BINARY ?= operator-sdk

# IMAGE_TAG_BASE defines the quay.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# quay.io/eclipse/che-operator-bundle:$VERSION and quay.io/eclipse/che-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= quay.io/eclipse/che-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= quay.io/eclipse/che-operator:next
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
CRD_BETA_OPTIONS ?= "crd:trivialVersions=true,crdVersions=v1beta1"

OPERATOR_YAML="config/manager/manager.yaml"

ENV_FILE="/tmp/che-operator-debug.env"
ECLIPSE_CHE_NAMESPACE="eclipse-che"

CRD_FOLDER="config/crd/bases"

ECLIPSE_CHE_CR=config/samples/org.eclipse.che_v1_checluster.yaml

# legacy crd v1beta1 file names
ECLIPSE_CHE_CRD_V1BETA1="$(CRD_FOLDER)/org_v1_che_crd-v1beta1.yaml"
ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1="$(CRD_FOLDER)/org.eclipse.che_chebackupserverconfigurations_crd-v1beta1.yaml"
ECLIPSE_CHE_BACKUP_CRD_V1BETA1="$(CRD_FOLDER)/org.eclipse.che_checlusterbackups_crd-v1beta1.yaml"
ECLIPSE_CHE_RESTORE_CRD_V1BETA1="$(CRD_FOLDER)/org.eclipse.che_checlusterrestores_crd-v1beta1.yaml"

# legacy crd file names
ECLIPSE_CHE_CRD_V1="$(CRD_FOLDER)/org_v1_che_crd.yaml"
ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1="$(CRD_FOLDER)/org.eclipse.che_chebackupserverconfigurations_crd.yaml"
ECLIPSE_CHE_BACKUP_CRD_V1="$(CRD_FOLDER)/org.eclipse.che_checlusterbackups_crd.yaml"
ECLIPSE_CHE_RESTORE_CRD_V1="$(CRD_FOLDER)/org.eclipse.che_checlusterrestores_crd.yaml"

# default crd names used operator-sdk from the box
ECLIPSE_CHE_CRD="$(CRD_FOLDER)/org.eclipse.che_checlusters.yaml"
ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD="$(CRD_FOLDER)/org.eclipse.che_chebackupserverconfigurations.yaml"
ECLIPSE_CHE_BACKUP_CRD="$(CRD_FOLDER)/org.eclipse.che_checlusterbackups.yaml"
ECLIPSE_CHE_RESTORE_CRD="$(CRD_FOLDER)/org.eclipse.che_checlusterrestores.yaml"

DEV_WORKSPACE_CONTROLLER_VERSION="v0.11.0"
DEV_HEADER_REWRITE_TRAEFIK_PLUGIN="main"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
.ONESHELL:

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

download-operator-sdk:
	ARCH=$$(case "$$(uname -m)" in
	x86_64) echo -n amd64 ;;
	aarch64) echo -n arm64 ;;
	*) echo -n $$(uname -m)
	esac)
	OS=$$(uname | awk '{print tolower($$0)}')

	OPERATOR_SDK_VERSION=$$(sed -r 's|operator-sdk:\s*(.*)|\1|' REQUIREMENTS)

	echo "[INFO] ARCH: $$ARCH, OS: $$OS. operator-sdk version: $$OPERATOR_SDK_VERSION"

	if [ -z $(OP_SDK_DIR) ]; then
		OP_SDK_PATH="operator-sdk"
	else
		OP_SDK_PATH="$(OP_SDK_DIR)/operator-sdk"
	fi

	echo "[INFO] Downloading operator-sdk..."

	OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/$${OPERATOR_SDK_VERSION}
	curl -sSLo $${OP_SDK_PATH} $${OPERATOR_SDK_DL_URL}/operator-sdk_$${OS}_$${ARCH}

	echo "[INFO] operator-sdk will downloaded to: $${OP_SDK_PATH}"
	echo "[INFO] Set up executable permissions to binary."
	chmod +x $${OP_SDK_PATH}
	echo "[INFO] operator-sdk is ready."

removeRequiredAttribute: SHELL := /bin/bash
removeRequiredAttribute:
	REQUIRED=false

	while IFS= read -r line
	do
		if [[ $$REQUIRED == true ]]; then
			if [[ $$line == *"- "* ]]; then
				continue
			else
				REQUIRED=false
			fi
		fi

		if [[ $$line == *"required:"* ]]; then
			REQUIRED=true
			continue
		fi

		echo  "$$line" >> $$filePath.tmp
	done < "$$filePath"

	mv $${filePath}.tmp $${filePath}

manifests: controller-gen add-license-download ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	# Generate CRDs v1beta1
	$(CONTROLLER_GEN) $(CRD_BETA_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	mv "$(ECLIPSE_CHE_CRD)" "$(ECLIPSE_CHE_CRD_V1BETA1)"
	mv "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD)" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	mv "$(ECLIPSE_CHE_BACKUP_CRD)" "$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	mv "$(ECLIPSE_CHE_RESTORE_CRD)" "$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"

	# Generate CRDs v1
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	mv "$(ECLIPSE_CHE_CRD)" "$(ECLIPSE_CHE_CRD_V1)"
	mv "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD)" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1)"
	mv "$(ECLIPSE_CHE_BACKUP_CRD)" "$(ECLIPSE_CHE_BACKUP_CRD_V1)"
	mv "$(ECLIPSE_CHE_RESTORE_CRD)" "$(ECLIPSE_CHE_RESTORE_CRD_V1)"

	# remove yaml delimitier, which makes OLM catalog source image broken.
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_CRD_V1BETA1)"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"
	rm -rf "$(ECLIPSE_CHE_CRD_V1BETA1).bak" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1).bak" "$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1).bak" "$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1).bak"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_CRD_V1)"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1)"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_BACKUP_CRD_V1)"
	sed -i.bak '/---/d' "$(ECLIPSE_CHE_RESTORE_CRD_V1)"
	rm -rf "$(ECLIPSE_CHE_CRD_V1).bak" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1).bak" "$(ECLIPSE_CHE_BACKUP_CRD_V1).bak" "$(ECLIPSE_CHE_RESTORE_CRD_V1).bak"

	# remove v1alphav2 version from crd files
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_CRD_V1BETA1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_CRD_V1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_BACKUP_CRD_V1)"
	yq -rYi "del(.spec.versions[1])" "$(ECLIPSE_CHE_RESTORE_CRD_V1)"

	# remove .spec.subresources.status from crd v1beta1 files
	yq -rYi ".spec.subresources.status = {}" "$(ECLIPSE_CHE_CRD_V1BETA1)"
	yq -rYi ".spec.subresources.status = {}" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	yq -rYi ".spec.subresources.status = {}" "$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	yq -rYi ".spec.subresources.status = {}" "$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"

	# remove .spec.validation.openAPIV3Schema.type field
	yq -rYi "del(.spec.validation.openAPIV3Schema.type)" "$(ECLIPSE_CHE_CRD_V1BETA1)"
	yq -rYi "del(.spec.validation.openAPIV3Schema.type)" "$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	yq -rYi "del(.spec.validation.openAPIV3Schema.type)" "$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	yq -rYi "del(.spec.validation.openAPIV3Schema.type)" "$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"

	# remove "required" attributes from v1beta1 crd files
	$(MAKE) removeRequiredAttribute "filePath=$(ECLIPSE_CHE_CRD_V1BETA1)"
	$(MAKE) removeRequiredAttribute "filePath=$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	$(MAKE) removeRequiredAttribute "filePath=$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	$(MAKE) removeRequiredAttribute "filePath=$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"

	$(MAKE) add-license $$(find ./config/crd -not -path "./vendor/*" -name "*.yaml")

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

compile:
	binary="$(BINARY)"
	if [ -z "$${binary}" ]; then
		binary="/tmp/che-operator/che-operator"
	fi
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o "$${binary}" main.go
	echo "che-operator binary compiled to $${binary}"

fmt: add-license-download ## Run go fmt against code.
  ifneq ($(shell command -v goimports 2> /dev/null),)
	  find . -not -path "./vendor/*" -name "*.go" -exec goimports -w {} \;
  else
	  @echo "WARN: goimports is not installed -- formatting using go fmt instead."
	  @echo "      Please install goimports to ensure file imports are consistent."
	  go fmt -x ./...
  endif

	FILES_TO_CHECK_LICENSE=$$(find . \
		-not -path "./mocks/*" \
		-not -path "./vendor/*" \
		-not -path "./testbin/*" \
		-not -path "./bundle/stable/*" \
		-not -path "./config/manager/controller_manager_config.yaml" \
		\( -name '*.sh' -o -name "*.go" -o -name "*.yaml" -o -name "*.yml" \))

	for f in $${FILES_TO_CHECK_LICENSE}
	do
		$(MAKE) add-license $${f}
	done

vet: ## Run go vet against code.
	go vet ./...

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet prepare-templates ## Run tests.
	export MOCK_API=true; go test -mod=vendor ./... -coverprofile cover.out

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

IMAGE_TOOL=docker

docker-build: ## Build docker image with the manager.
	${IMAGE_TOOL} build -t ${IMG} .

docker-push: ## Push docker image with the manager.
	${IMAGE_TOOL} push ${IMG}

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager || true && $(KUSTOMIZE) edit set image quay.io/eclipse/che-operator:next=${IMG} && cd ../..
	$(KUSTOMIZE) build config/default | kubectl apply -f -

	echo "[INFO] Start printing logs..."
	oc wait --for=condition=ready pod -l app.kubernetes.io/component=che-operator -n ${ECLIPSE_CHE_NAMESPACE} --timeout=60s
	oc logs $$(oc get pods -o json -n ${ECLIPSE_CHE_NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name') -n ${ECLIPSE_CHE_NAMESPACE} --all-containers -f

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

prepare-templates:
	# Download Dev Workspace operator templates
	echo "[INFO] Downloading Dev Workspace operator templates ..."
	rm -f /tmp/devworkspace-operator.zip
	rm -rf /tmp/devfile-devworkspace-operator-*
	rm -rf /tmp/devworkspace-operator/
	mkdir -p /tmp/devworkspace-operator/templates

	curl -sL https://api.github.com/repos/devfile/devworkspace-operator/zipball/${DEV_WORKSPACE_CONTROLLER_VERSION} > /tmp/devworkspace-operator.zip

	unzip -q /tmp/devworkspace-operator.zip '*/deploy/deployment/*' -d /tmp
	cp -rf /tmp/devfile-devworkspace-operator*/deploy/* /tmp/devworkspace-operator/templates
	echo "[INFO] Downloading Dev Workspace operator templates completed."

	echo "[INFO] Downloading Gateway plugin resources ..."
	rm -f /tmp/asset-header-rewrite-traefik-plugin.zip
	rm -rf /tmp/header-rewrite-traefik-plugin
	rm -rf /tmp/*-header-rewrite-traefik-plugin-*/
	curl -sL https://api.github.com/repos/che-incubator/header-rewrite-traefik-plugin/zipball/${DEV_HEADER_REWRITE_TRAEFIK_PLUGIN} > /tmp/asset-header-rewrite-traefik-plugin.zip

	unzip -q /tmp/asset-header-rewrite-traefik-plugin.zip -d /tmp
	mkdir -p /tmp/header-rewrite-traefik-plugin
	mv /tmp/*-header-rewrite-traefik-plugin-*/headerRewrite.go /tmp/*-header-rewrite-traefik-plugin-*/.traefik.yml /tmp/header-rewrite-traefik-plugin
	echo "[INFO] Downloading Gateway plugin resources completed."

create-namespace:
	set +e
	kubectl create namespace ${ECLIPSE_CHE_NAMESPACE} || true
	set -e

apply-crd:
	kubectl apply -f ${ECLIPSE_CHE_CRD_V1}
	kubectl apply -f ${ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1}
	kubectl apply -f ${ECLIPSE_CHE_BACKUP_CRD_V1}
	kubectl apply -f ${ECLIPSE_CHE_RESTORE_CRD_V1}

.PHONY: init-cr
init-cr:
	if [ "$$(oc get checluster -n ${ECLIPSE_CHE_NAMESPACE} eclipse-che || false )" ]; then
		echo "Che Cluster already exists. Using it."
	else
	echo "Che Cluster is not found. Creating a new one from $(ECLIPSE_CHE_CR)"
		# before applying resources on K8s check if ingress domain corresponds to the current cluster
		# no OpenShift ingress domain is ignored, so skip it
		if [ "$$(oc api-resources --api-group='route.openshift.io' 2>&1 | grep -o routes)" != "routes" ]; then
			export CLUSTER_API_URL=$$(oc whoami --show-server=true) || true;
			export CLUSTER_DOMAIN=$$(echo $${CLUSTER_API_URL} | sed -E 's/https:\/\/(.*):.*/\1/g')
			export CHE_CLUSTER_DOMAIN=$$(yq -r .spec.k8s.ingressDomain $(ECLIPSE_CHE_CR))
			export CHE_CLUSTER_DOMAIN=$${CHE_CLUSTER_DOMAIN%".nip.io"}
			if [ $${CLUSTER_DOMAIN} != $${CHE_CLUSTER_DOMAIN} ];then
				echo "[WARN] Your cluster address is $${CLUSTER_DOMAIN} but CheCluster has $${CHE_CLUSTER_DOMAIN} configured"
				echo "[WARN] Make sure that .spec.k8s.ingressDomain in $${ECLIPSE_CHE_CR} has the right value and rerun"
				echo "[WARN] Press y to continue anyway. [y/n] ? " && read ans && [ $${ans:-N} = y ] || exit 1;
			fi
		fi
		kubectl apply -f ${ECLIPSE_CHE_CR} -n ${ECLIPSE_CHE_NAMESPACE}
	fi

apply-cr-crd-beta:
	kubectl apply -f ${ECLIPSE_CHE_CRD_V1BETA1}
	kubectl apply -f ${ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1}
	kubectl apply -f ${ECLIPSE_CHE_BACKUP_CRD_V1BETA1}
	kubectl apply -f ${ECLIPSE_CHE_RESTORE_CRD_V1BETA1}
	kubectl apply -f ${ECLIPSE_CHE_CR} -n ${ECLIPSE_CHE_NAMESPACE}

create-env-file: prepare-templates
	rm -rf "${ENV_FILE}"
	touch "${ENV_FILE}"
	CLUSTER_API_URL=$$(oc whoami --show-server=true) || true;
	if [ -n $${CLUSTER_API_URL} ]; then
		echo "CLUSTER_API_URL='$${CLUSTER_API_URL}'" >> "${ENV_FILE}"
		echo "[INFO] Set up cluster api url: $${CLUSTER_API_URL}"
	fi;
	echo "WATCH_NAMESPACE='${ECLIPSE_CHE_NAMESPACE}'" >> "${ENV_FILE}"

create-full-env-file: create-env-file
	cat ./$(OPERATOR_YAML) | \
	yq -r '.spec.template.spec.containers[0].env[] | select(.name == "WATCH_NAMESPACE" | not) | "export \(.name)=\"\(.value)\""' \
	>> ${ENV_FILE}
	echo "[INFO] Env file: ${ENV_FILE}"
	source ${ENV_FILE} ; env | grep CHE_VERSION

debug: generate manifests kustomize prepare-templates create-namespace apply-crd init-cr create-env-file
	echo "[WARN] Make sure that your CR contains valid ingress domain!"
	# dlv has an issue with 'Ctrl-C' termination, that's why we're doing trick with detach.
	dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go -- &
	OPERATOR_SDK_PID=$!
	echo "[INFO] Use 'make uninstall' to remove Che installation after debug"
	wait $$OPERATOR_SDK_PID

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

ADD_LICENSE = $(shell pwd)/bin/addlicense
add-license-download: ## Download addlicense locally if necessary.
	$(call go-get-tool,$(ADD_LICENSE),github.com/google/addlicense@99ebc9c9db7bceb8623073e894533b978d7b7c8a)

add-license:
	# Get all argument and remove make goal("add-license") to get only list files
	FILES=$$(echo $(filter-out $@,$(MAKECMDGOALS)))
	$(ADD_LICENSE) -f hack/license-header.txt $${FILES}

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "[INFO] Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

update-roles:
	echo "[INFO] Updating roles with DW roles"

	CLUSTER_ROLES=(
		https://raw.githubusercontent.com/devfile/devworkspace-operator/${DEV_WORKSPACE_CONTROLLER_VERSION}/deploy/deployment/openshift/objects/devworkspace-controller-view-workspaces.ClusterRole.yaml
		https://raw.githubusercontent.com/devfile/devworkspace-operator/${DEV_WORKSPACE_CONTROLLER_VERSION}/deploy/deployment/openshift/objects/devworkspace-controller-edit-workspaces.ClusterRole.yaml
		https://raw.githubusercontent.com/devfile/devworkspace-operator/${DEV_WORKSPACE_CONTROLLER_VERSION}/deploy/deployment/openshift/objects/devworkspace-controller-leader-election-role.Role.yaml
		https://raw.githubusercontent.com/devfile/devworkspace-operator/${DEV_WORKSPACE_CONTROLLER_VERSION}/deploy/deployment/openshift/objects/devworkspace-controller-proxy-role.ClusterRole.yaml
		https://raw.githubusercontent.com/devfile/devworkspace-operator/${DEV_WORKSPACE_CONTROLLER_VERSION}/deploy/deployment/openshift/objects/devworkspace-controller-role.ClusterRole.yaml
		https://raw.githubusercontent.com/devfile/devworkspace-operator/${DEV_WORKSPACE_CONTROLLER_VERSION}/deploy/deployment/openshift/objects/devworkspace-controller-metrics-reader.ClusterRole.yaml
	)

	# Updates cluster_role.yaml based on DW roles
	## Removes old cluster roles
	cat config/rbac/cluster_role.yaml | sed '/CHE-OPERATOR ROLES ONLY: END/q0' > config/rbac/cluster_role.yaml.tmp
	mv config/rbac/cluster_role.yaml.tmp config/rbac/cluster_role.yaml

	# Copy new cluster roles
	for roles in "$${CLUSTER_ROLES[@]}"; do
		echo "  # "$$(basename $$roles) >> config/rbac/cluster_role.yaml

		CONTENT=$$(curl -sL $$roles | sed '1,/rules:/d')
		while IFS= read -r line; do
			echo "  $$line" >> config/rbac/cluster_role.yaml
		done <<< "$$CONTENT"
	done

	ROLES=(
		# currently, there are no other roles we need to incorporate
	)

	# Updates role.yaml
	## Removes old roles
	cat config/rbac/role.yaml | sed '/CHE-OPERATOR ROLES ONLY: END/q0' > config/rbac/role.yaml.tmp
	mv config/rbac/role.yaml.tmp config/rbac/role.yaml

	## Copy new roles
	for roles in "$${ROLES[@]}"; do
		echo "# "$$(basename $$roles) >> config/rbac/role.yaml

		CONTENT=$$(curl -sL $$roles | sed '1,/rules:/d')
		while IFS= read -r line; do
			echo "$$line" >> config/rbac/role.yaml
		done <<< "$$CONTENT"
	done

.PHONY: bundle
bundle: generate manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	if [ -z "$(channel)" ]; then
		echo "[ERROR] 'channel' is not specified."
		exit 1
	fi

	if [ -z "$(NO_INCREMENT)" ]; then
		$(MAKE) increment-next-version
	fi

	echo "[INFO] Updating OperatorHub bundle"

	BUNDLE_PATH=$$($(MAKE) getBundlePath channel="$${channel}" -s)
	NEW_CSV=$${BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml
	newNextBundleVersion=$$(yq -r ".spec.version" "$${NEW_CSV}")
	echo "[INFO] Creation new next bundle version: $${newNextBundleVersion}"

	createdAtOld=$$(yq -r ".metadata.annotations.createdAt" "$${NEW_CSV}")

	BUNDLE_PACKAGE=$$($(MAKE) getPackageName)
	BUNDLE_DIR="bundle/"$${channel}"/$${BUNDLE_PACKAGE}"
	GENERATED_CSV_NAME=$${BUNDLE_PACKAGE}.clusterserviceversion.yaml
	DESIRED_CSV_NAME=che-operator.clusterserviceversion.yaml
	GENERATED_CRD_NAME=org.eclipse.che_checlusters.yaml
	DESIRED_CRD_NAME=org_v1_che_crd.yaml

	$(OPERATOR_SDK_BINARY) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image quay.io/eclipse/che-operator:next=$(IMG) && cd ../..
	$(KUSTOMIZE) build config/platforms/openshift | \
	$(OPERATOR_SDK_BINARY) generate bundle \
	--quiet \
	--overwrite \
	--version $${newNextBundleVersion} \
	--package $${BUNDLE_PACKAGE} \
	--output-dir $${BUNDLE_DIR} \
	--channels $(channel) \
	--default-channel $(channel)

	rm -rf bundle.Dockerfile

	cd $${BUNDLE_DIR}/manifests;
	mv $${GENERATED_CSV_NAME} $${DESIRED_CSV_NAME}
	mv $${GENERATED_CRD_NAME} $${DESIRED_CRD_NAME}
	cd $(mkfile_dir)

	$(OPERATOR_SDK_BINARY) bundle validate ./$${BUNDLE_DIR}

	containerImage=$$(sed -n 's|^ *image: *\([^ ]*/che-operator:[^ ]*\) *|\1|p' $${NEW_CSV})
	echo "[INFO] Updating new package version fields:"
	echo "[INFO]        - containerImage => $${containerImage}"
	sed -e "s|containerImage:.*$$|containerImage: $${containerImage}|" "$${NEW_CSV}" > "$${NEW_CSV}.new"
	mv "$${NEW_CSV}.new" "$${NEW_CSV}"

	if [ "$(NO_DATE_UPDATE)" = true ]; then
		echo "[INFO]        - createdAt => $${createdAtOld}"
		sed -e "s/createdAt:.*$$/createdAt: \"$${createdAtOld}\"/" "$${NEW_CSV}" > "$${NEW_CSV}.new"
		mv "$${NEW_CSV}.new" "$${NEW_CSV}"
	fi

	CRD="$${BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
	yq -riY  '.spec.preserveUnknownFields = false' $${CRD}

	if [ -n "$(TAG)" ]; then
		echo "[INFO] Set tags in next OLM files"
		sed -ri "s/(.*:\s?)$(RELEASE)([^-])?$$/\1$(TAG)\2/" "$${NEW_CSV}"
	fi

	# Remove roles for openshift bundle
	YAML_CONTENT=$$(cat "$${NEW_CSV}")
	clusterPermLength=$$(echo "$${YAML_CONTENT}" | yq -r ".spec.install.spec.clusterPermissions[0].rules | length")
	i=0
	while [ "$${i}" -lt "$${clusterPermLength}" ]; do
		apiGroupLength=$$(echo "$${YAML_CONTENT}" | yq -r '.spec.install.spec.clusterPermissions[0].rules['$${i}'].apiGroups | length')
		if [ "$${apiGroupLength}" -gt 0 ]; then
			j=0
			while [ "$${j}" -lt "$${apiGroupLength}" ]; do
				apiGroup=$$(echo "$${YAML_CONTENT}" | yq -r '.spec.install.spec.clusterPermissions[0].rules['$${i}'].apiGroups['$${j}']')
				case $${apiGroup} in cert-manager.io)
					YAML_CONTENT=$$(echo "$${YAML_CONTENT}" | yq -rY 'del(.spec.install.spec.clusterPermissions[0].rules['$${i}'])' )
					j=$$((j-1))
					i=$$((i-1))
					break
					;;
				esac;
				j=$$((i+1))
			done
		fi
		i=$$((i+1))
	done
	echo "$${YAML_CONTENT}" > "$${NEW_CSV}"

	# Removes che-tls-secret-creator
	index=0
	while [ $${index} -le 30 ]
	do
		if [ $$(cat $${NEW_CSV} | yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env['$${index}'].name') = "RELATED_IMAGE_che_tls_secrets_creation_job" ]; then
			yq -rYSi 'del(.spec.install.spec.deployments[0].spec.template.spec.containers[0].env['$${index}'])' $${NEW_CSV}
			break
		fi
		index=$$((index+1))
	done

	# Fix CSV
	echo "[INFO] Fix CSV"
	fixedSample=$$(yq -r ".metadata.annotations[\"alm-examples\"] | \
		fromjson | \
		del( .[] | select(.kind == \"CheCluster\") | .spec.k8s)" $${NEW_CSV} |  sed -r 's/"/\\"/g')

	yq -riY ".metadata.annotations[\"alm-examples\"] = \"$${fixedSample}\"" $${NEW_CSV}

	# set `app.kubernetes.io/managed-by` label
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.metadata.labels."app.kubernetes.io/managed-by") = "olm"' "$${NEW_CSV}"

	# set Pod Security Context Posture
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostIPC") = false' "$${NEW_CSV}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostNetwork") = false' "$${NEW_CSV}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostPID") = false' "$${NEW_CSV}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[0].securityContext."allowPrivilegeEscalation") = false' "$${NEW_CSV}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[0].securityContext."runAsNonRoot") = true' "$${NEW_CSV}"

	printf "\n  com.redhat.openshift.versions: \"v4.8\"" >> $${BUNDLE_PATH}/metadata/annotations.yaml

	# Base cluster service version file has got correctly sorted CRDs.
	# They are sorted with help of annotation markers in the api type files ("api/v1" folder).
	# Example such annotation: +operator-sdk:csv:customresourcedefinitions:order=0
	# Let's copy this sorted CRDs to the bundle cluster service version file.
	BASE_CSV="config/manifests/bases/che-operator.clusterserviceversion.yaml"
	CRD_API=$$(yq -c '.spec.customresourcedefinitions.owned' $${BASE_CSV})
	yq -riSY ".spec.customresourcedefinitions.owned = $$CRD_API" "$${NEW_CSV}"
	yq -riSY "del(.spec.customresourcedefinitions.owned[] | select(.version == \"v2alpha1\"))" "$${NEW_CSV}"

	# Format code.
	yq -rY "." "$${NEW_CSV}" > "$${NEW_CSV}.old"
	mv "$${NEW_CSV}.old" "$${NEW_CSV}"

	$(MAKE) add-license $$(find $${BUNDLE_PATH} -name "*.yaml")
	$(MAKE) add-license $${BASE_CSV}

getPackageName:
	echo "eclipse-che-preview-openshift"

getBundlePath:
	if [ -z "$(channel)" ]; then
		echo "[ERROR] 'channel' is not specified"
		exit 1
	fi
	PACKAGE_NAME=$$($(MAKE) getPackageName)
	echo "$(PROJECT_DIR)/bundle/$(channel)/$${PACKAGE_NAME}"

increment-next-version:
	if [ -z "$(channel)" ]; then
		echo "[ERROR] 'channel' is not specified"
		exit 1
	fi

	BUNDLE_PATH=$$($(MAKE) getBundlePath channel="$(channel)" -s)
	OPM_BUNDLE_MANIFESTS_DIR="$${BUNDLE_PATH}/manifests"
	CSV="$${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

	currentNextVersion=$$(yq -r ".spec.version" "$${CSV}")
	echo  "[INFO] Current next version: $${currentNextVersion}"

	incrementPart=$$($(MAKE) get-next-version-increment nextVersion="$${currentNextVersion}" -s)

	PACKAGE_NAME=$$($(MAKE) getPackageName)

	CLUSTER_SERVICE_VERSION=$$($(MAKE) get-current-stable-version)
	STABLE_PACKAGE_VERSION=$$(echo "$${CLUSTER_SERVICE_VERSION}" | sed -e "s/$${PACKAGE_NAME}.v//")
	echo "[INFO] Current stable package version: $${STABLE_PACKAGE_VERSION}"

	# Parse stable version parts
	majorAndMinor=$${STABLE_PACKAGE_VERSION%.*}
	STABLE_MINOR_VERSION=$${majorAndMinor#*.}
	STABLE_MAJOR_VERSION=$${majorAndMinor%.*}

	STABLE_MINOR_VERSION=$$(($$STABLE_MINOR_VERSION+1))
	echo "$${STABLE_MINOR_VERSION}"

	incrementPart=$$((incrementPart+1))
	newVersion="$${STABLE_MAJOR_VERSION}.$${STABLE_MINOR_VERSION}.0-$${incrementPart}.$(channel)"

	echo "[INFO] Set up next version: $${newVersion}"
	yq -rY "(.spec.version) = \"$${newVersion}\" | (.metadata.name) = \"$${PACKAGE_NAME}.v$${newVersion}\"" "$${CSV}" > "$${CSV}.old"
	mv "$${CSV}.old" "$${CSV}"

get-current-stable-version:
	STABLE_BUNDLE_PATH=$$($(MAKE) getBundlePath channel="stable" -s)
	LAST_STABLE_CSV="$${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"

	lastStableVersion=$$(yq -r ".spec.version" "$${LAST_STABLE_CSV}")
	echo "$${lastStableVersion}"

get-next-version-increment:
	if [ -z $(nextVersion) ]; then
		echo "[ERROR] Provide next version to parse"
		exit 1
	fi

	versionWithoutNext="$${nextVersion%.next*}"
	version="$${versionWithoutNext%-*}"
	incrementPart="$${versionWithoutNext#*-}"
	echo "$${incrementPart}"

update-resources: SHELL := /bin/bash
update-resources: check-requirements update-resource-images update-roles
	$(MAKE) bundle channel=next
	$(MAKE) update-helmcharts

update-helmcharts: SHELL := /bin/bash
update-helmcharts: add-license-download check-requirements
	helmFolder=$(HELM_FOLDER)
	if [ -z "$${helmFolder}" ]; then
		helmFolder="next"
	fi
	HELMCHARTS_TEMPLATES="helmcharts/$${helmFolder}/templates"
	HELMCHARTS_CRDS="helmcharts/$${helmFolder}/crds"

	echo "[INFO] Update Helm templates $${HELMCHARTS_TEMPLATES}"
	cp config/manager/manager.yaml $${HELMCHARTS_TEMPLATES}
	cp config/rbac/cluster_role.yaml $${HELMCHARTS_TEMPLATES}
	cp config/rbac/cluster_rolebinding.yaml $${HELMCHARTS_TEMPLATES}
	cp config/rbac/service_account.yaml $${HELMCHARTS_TEMPLATES}
	cp config/rbac/role.yaml $${HELMCHARTS_TEMPLATES}
	cp config/rbac/role_binding.yaml $${HELMCHARTS_TEMPLATES}
	cp config/samples/org.eclipse.che_v1_checluster.yaml $${HELMCHARTS_TEMPLATES}

	echo "[INFO] Update helm CRDs $${HELMCHARTS_CRDS}"
	cp config/crd/bases/org_v1_che_crd.yaml $${HELMCHARTS_CRDS}
	cp config/crd/bases/org.eclipse.che_chebackupserverconfigurations_crd.yaml $${HELMCHARTS_CRDS}
	cp config/crd/bases/org.eclipse.che_checlusterbackups_crd.yaml $${HELMCHARTS_CRDS}
	cp config/crd/bases/org.eclipse.che_checlusterrestores_crd.yaml $${HELMCHARTS_CRDS}

	yq -riY ".metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\"" $${HELMCHARTS_TEMPLATES}/manager.yaml
	yq -riY ".metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\"" $${HELMCHARTS_TEMPLATES}/service_account.yaml
	yq -riY ".metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\"" $${HELMCHARTS_TEMPLATES}/role.yaml
	yq -riY ".metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\"" $${HELMCHARTS_TEMPLATES}/role_binding.yaml
	yq -riY ".subjects[0].namespace = \"$(ECLIPSE_CHE_NAMESPACE)\"" $${HELMCHARTS_TEMPLATES}/cluster_rolebinding.yaml

	if [ $${helmFolder} == "stable" ]; then
		chartYaml="helmcharts/$${helmFolder}/Chart.yaml"

		EXAMPLE_FILES=(
			$${HELMCHARTS_TEMPLATES}/org.eclipse.che_v1_checluster.yaml \
			config/samples/org_v1_checlusterbackup.yaml \
			config/samples/org_v1_checlusterrestore.yaml \
			config/samples/org_v1_chebackupserverconfiguration.yaml
		)

		CRDS=""
		for exampleFile in "$${EXAMPLE_FILES[@]}"; do
			example=$$(cat $${exampleFile} | yq -rY ". | (.metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\") | [.]")
		 	CRDS=$${CRDS}$${example}$$'\n'
		done

		yq -rYi --arg examples "$${CRDS}" ".annotations.\"artifacthub.io/crdsExamples\" = \$$examples" $${chartYaml}
		rm -rf $${HELMCHARTS_TEMPLATES}/org.eclipse.che_v1_checluster.yaml
	else
		# Set references to values
		yq -riY ".metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\"" $${HELMCHARTS_TEMPLATES}/org.eclipse.che_v1_checluster.yaml
		yq -riY ".spec.k8s.ingressDomain |= \"{{ .Values.k8s.ingressDomain }}\"" $${HELMCHARTS_TEMPLATES}/org.eclipse.che_v1_checluster.yaml
	fi

	$(MAKE) add-license $$(find ./helmcharts -name "*.yaml")

check-requirements:
	. olm/check-yq.sh

	DOCKER=$$(command -v docker || true)
	if [[ ! -x $$DOCKER ]]; then
		echo "[ERROR] "docker" is not installed."
		exit 1
	fi

	SKOPEO=$$(command -v skopeo || true)
	if [[ ! -x $$SKOPEO ]]; then
		echo "[ERROR] "scopeo" is not installed."
		exit 1
	fi

	OPERATOR_SDK_BINARY=$(OPERATOR_SDK_BINARY)
	if [ -z "$${OPERATOR_SDK_BINARY}" ]; then
		OPERATOR_SDK_BINARY=$$(command -v operator-sdk)
		if [[ ! -x "$${OPERATOR_SDK_BINARY}" ]]; then
			echo "[ERROR] operator-sdk is not installed."
			exit 1
		fi
	fi

	operatorVersion=$$($${OPERATOR_SDK_BINARY} version)
	REQUIRED_OPERATOR_SDK=$$(yq -r ".\"operator-sdk\"" "REQUIREMENTS")
	case "$$operatorVersion" in
		*$$REQUIRED_OPERATOR_SDK*) ;;
		*) echo "[ERROR] operator-sdk $${REQUIRED_OPERATOR_SDK} is required"; exit 1 ;;
	esac

update-deployment-yaml-images: add-license-download
	if [ -z $(UBI8_MINIMAL_IMAGE) ] || [ -z $(PLUGIN_BROKER_METADATA_IMAGE) ] || [ -z $(PLUGIN_BROKER_ARTIFACTS_IMAGE) ] || [ -z $(JWT_PROXY_IMAGE) ]; then
		echo "[ERROR] Define required arguments: `UBI8_MINIMAL_IMAGE`, `PLUGIN_BROKER_METADATA_IMAGE`, `PLUGIN_BROKER_ARTIFACTS_IMAGE`, `JWT_PROXY_IMAGE`"
		exit 1
	fi
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_pvc_jobs\") | .value ) = \"$(UBI8_MINIMAL_IMAGE)\"" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_metadata\") | .value ) = \"$(PLUGIN_BROKER_METADATA_IMAGE)\"" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_artifacts\") | .value ) = \"$(PLUGIN_BROKER_ARTIFACTS_IMAGE)\"" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"$(JWT_PROXY_IMAGE)\"" $(OPERATOR_YAML)

	$(MAKE) add-license $(OPERATOR_YAML)

update-dockerfile-image:
	if [ -z $(UBI8_MINIMAL_IMAGE) ]; then
		echo "[ERROR] Define `UBI8_MINIMAL_IMAGE` argument"
	fi
	DOCKERFILE="Dockerfile"
	sed -i 's|registry.access.redhat.com/ubi8-minimal:[^\s]* |'${UBI8_MINIMAL_IMAGE}' |g' $${DOCKERFILE}

update-resource-images:
	# Detect newer images
	echo "[INFO] Check update some base images..."
	ubiMinimal8Version=$$(skopeo --override-os linux inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.version')
	ubiMinimal8Release=$$(skopeo --override-os linux inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.release')
	UBI8_MINIMAL_IMAGE="registry.access.redhat.com/ubi8-minimal:$${ubiMinimal8Version}-$${ubiMinimal8Release}"
	skopeo --override-os linux inspect docker://$${UBI8_MINIMAL_IMAGE} > /dev/null

	echo "[INFO] Check update broker and jwt proxy images..."
	wget https://raw.githubusercontent.com/eclipse-che/che-server/main/assembly/assembly-wsmaster-war/src/main/webapp/WEB-INF/classes/che/che.properties -q -O /tmp/che.properties
	PLUGIN_BROKER_METADATA_IMAGE=$$(cat /tmp/che.properties| grep "che.workspace.plugin_broker.metadata.image" | cut -d = -f2)
	PLUGIN_BROKER_ARTIFACTS_IMAGE=$$(cat /tmp/che.properties | grep "che.workspace.plugin_broker.artifacts.image" | cut -d = -f2)
	JWT_PROXY_IMAGE=$$(cat /tmp/che.properties | grep "che.server.secure_exposer.jwtproxy.image" | cut -d = -f2)

	echo "[INFO] UBI base image               : $${UBI8_MINIMAL_IMAGE}"
	echo "[INFO] Plugin broker metadata image : $${PLUGIN_BROKER_METADATA_IMAGE}"
	echo "[INFO] Plugin broker artifacts image: $${PLUGIN_BROKER_ARTIFACTS_IMAGE}"
	echo "[INFO] Plugin broker jwt proxy image: $${JWT_PROXY_IMAGE}"

	# Update operator deployment images.
	$(MAKE) update-deployment-yaml-images \
	UBI8_MINIMAL_IMAGE="$${UBI8_MINIMAL_IMAGE}" \
	PLUGIN_BROKER_METADATA_IMAGE=$${PLUGIN_BROKER_METADATA_IMAGE} \
	PLUGIN_BROKER_ARTIFACTS_IMAGE=$${PLUGIN_BROKER_ARTIFACTS_IMAGE} \
	JWT_PROXY_IMAGE=$${JWT_PROXY_IMAGE}

	# Update che-operator Dockerfile
	$(MAKE) update-dockerfile-image UBI8_MINIMAL_IMAGE="$${UBI8_MINIMAL_IMAGE}"

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	if [ -z "$(channel)" ]; then
		echo "[ERROR] 'channel' is not specified"
		exit 1
	fi

	BUNDLE_PACKAGE=$$($(MAKE) getPackageName)
	BUNDLE_DIR="bundle/$(channel)/$${BUNDLE_PACKAGE}"
	cd $${BUNDLE_DIR}
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
	cd ../../..

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.2/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=quay.io/eclipse/operator-bundle:v0.1.0,quay.io/eclipse/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=quay.io/eclipse/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add \
	--build-tool $(IMAGE_TOOL) \
	--bundles $(BUNDLE_IMGS) \
	--tag $(CATALOG_IMG) \
	--pull-tool $(IMAGE_TOOL) \
	--binary-image=quay.io/operator-framework/upstream-opm-builder:v1.15.2 \
	--mode semver $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

chectl-templ: SHELL := /bin/bash
chectl-templ:
	if [ -z "$(TARGET)" ]; then
		echo "[ERROR] Specify templates target location, using argument `TARGET`"
		exit 1
	fi
	if [ -z "$(SRC)" ]; then
		SRC=$$(pwd)
	else
		SRC=$(SRC)
	fi

	mkdir -p $(TARGET)

	cp -f "$${SRC}/config/manager/manager.yaml" "$(TARGET)/operator.yaml"
	cp -rf "$${SRC}/config/crd/bases/" "$(TARGET)/crds/"
	cp -f "$${SRC}/config/rbac/role.yaml" "$(TARGET)/"
	cp -f "$${SRC}/config/rbac/role_binding.yaml" "$(TARGET)/"
	cp -f "$${SRC}/config/rbac/cluster_role.yaml" "$(TARGET)/"
	cp -f "$${SRC}/config/rbac/cluster_rolebinding.yaml" "$(TARGET)/"
	cp -f "$${SRC}/config/rbac/service_account.yaml" "$(TARGET)/"
	cp -f "$${SRC}/$(ECLIPSE_CHE_CR)" "$(TARGET)/crds/org_v1_che_cr.yaml"

	echo "[INFO] chectl template folder is ready: ${TARGET}"
