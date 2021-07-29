# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 1.0.2

CHANNELS = "nightly"

ifndef VERBOSE
MAKEFLAGS += --silent
endif

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir := $(dir $(mkfile_path))

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "preview,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=preview,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="preview,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

DEFAULT_CHANNEL = "nightly"

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

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

DEV_WORKSPACE_CONTROLLER_VERSION="main"
DEV_WORKSPACE_CHE_OPERATOR_VERSION="main"

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

ensure-license-header:
	if [ -z $(FILE) ]; then
		echo "[ERROR] Provide argument `FILE` with file path value."
		exit 1
	fi

	fileHeader=$$(head -10 $(FILE) | tr --delete '\n' | tr --delete '\r')
	licenseMarker="Copyright (c)"

	case "$${fileHeader}" in
		*$${licenseMarker}*) return ;;
	esac;

	echo "#
		#  Copyright (c) 2019-2021 Red Hat, Inc.
		#    This program and the accompanying materials are made
		#    available under the terms of the Eclipse Public License 2.0
		#    which is available at https://www.eclipse.org/legal/epl-2.0/
		#
		#  SPDX-License-Identifier: EPL-2.0
		#
		#  Contributors:
		#    Red Hat, Inc. - initial API and implementation" > $(FILE).tmp

	cat $(FILE) >> $(FILE).tmp
	mv $(FILE).tmp $(FILE)

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
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

	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_CRD_V1BETA1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1BETA1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_BACKUP_CRD_V1BETA1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_RESTORE_CRD_V1BETA1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_CRD_V1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_BACKUP_SERVER_CONFIGURATION_CRD_V1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_BACKUP_CRD_V1)"
	$(MAKE) ensure-license-header FILE="$(ECLIPSE_CHE_RESTORE_CRD_V1)"

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate fmt vet ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -mod=vendor ./... -coverprofile cover.out

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
	cd config/manager || true && $(KUSTOMIZE) edit set image controller=${IMG} && cd ../..
	$(KUSTOMIZE) build config/default | kubectl apply -f -

	echo "[INFO] Start printing logs..."
	oc wait --for=condition=ready pod -l app.kubernetes.io/component=che-operator -n ${ECLIPSE_CHE_NAMESPACE} --timeout=60s
	oc logs $$(oc get pods -o json -n ${ECLIPSE_CHE_NAMESPACE} | jq -r '.items[] | select(.metadata.name | test("che-operator-")).metadata.name') -n ${ECLIPSE_CHE_NAMESPACE} --all-containers -f

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

prepare-templates:
	echo "[INFO] Copying Che Operator ./templates ..."
	cp templates/keycloak-provision.sh /tmp/keycloak-provision.sh
	cp templates/delete-identity-provider.sh /tmp/delete-identity-provider.sh
	cp templates/create-github-identity-provider.sh /tmp/create-github-identity-provider.sh
	cp templates/oauth-provision.sh /tmp/oauth-provision.sh
	cp templates/keycloak-update.sh /tmp/keycloak-update.sh
	echo "[INFO] Copying Che Operator ./templates completed."

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

	# Download Dev Workspace Che operator templates
	echo "[INFO] Downloading Dev Workspace Che operator templates ..."
	rm -f /tmp/devworkspace-che-operator.zip
	rm -rf /tmp/che-incubator-devworkspace-che-operator-*
	rm -rf /tmp/devworkspace-che-operator/
	mkdir -p /tmp/devworkspace-che-operator/templates

	curl -sL https://api.github.com/repos/che-incubator/devworkspace-che-operator/zipball/${DEV_WORKSPACE_CHE_OPERATOR_VERSION} > /tmp/devworkspace-che-operator.zip

	unzip -q /tmp/devworkspace-che-operator.zip '*/deploy/deployment/*' -d /tmp
	cp -r /tmp/che-incubator-devworkspace-che-operator*/deploy/* /tmp/devworkspace-che-operator/templates
	echo "[INFO] Downloading Dev Workspace operator templates completed."

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
	cat ./config/manager/manager.yaml | \
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
	https://raw.githubusercontent.com/che-incubator/devworkspace-che-operator/main/deploy/deployment/openshift/objects/devworkspace-che-leader-election-role.Role.yaml
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
	if [ -z "$(platform)" ]; then
		echo "[INFO] You must specify 'platform' macros. For example: `make bundle platform=kubernetes`"
		exit 1
	fi

	if [ -z "$(NO_INCREMENT)" ]; then
		$(MAKE) increment-nightly-version platform="$${platform}"
	fi

	echo "[INFO] Updating OperatorHub bundle for platform '$${platform}'"

	NIGHTLY_BUNDLE_PATH=$$($(MAKE) getBundlePath platform="$${platform}" channel="nightly" -s)
	NEW_CSV=$${NIGHTLY_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml
	newNightlyBundleVersion=$$(yq -r ".spec.version" "$${NEW_CSV}")
	echo "[INFO] Creation new nightly bundle version: $${newNightlyBundleVersion}"

	createdAtOld=$$(yq -r ".metadata.annotations.createdAt" "$${NEW_CSV}")

	BUNDLE_PACKAGE="eclipse-che-preview-$(platform)"
	BUNDLE_DIR="bundle/$(DEFAULT_CHANNEL)/$${BUNDLE_PACKAGE}"
	GENERATED_CSV_NAME=$${BUNDLE_PACKAGE}.clusterserviceversion.yaml
	DESIRED_CSV_NAME=che-operator.clusterserviceversion.yaml
	GENERATED_CRD_NAME=org.eclipse.che_checlusters.yaml
	DESIRED_CRD_NAME=org_v1_che_crd.yaml

	$(OPERATOR_SDK_BINARY) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG) && cd ../..
	$(KUSTOMIZE) build config/platforms/$(platform) | \
	$(OPERATOR_SDK_BINARY) generate bundle \
	-q --overwrite \
	--version $${newNightlyBundleVersion} \
	--package $${BUNDLE_PACKAGE} \
	--output-dir $${BUNDLE_DIR} \
	$(BUNDLE_METADATA_OPTS)

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

	platformCRD="$${NIGHTLY_BUNDLE_PATH}/manifests/org_v1_che_crd.yaml"
	if [ "$${platform}" = "openshift" ]; then
		yq -riY  '.spec.preserveUnknownFields = false' $${platformCRD}
	fi
	$(MAKE) ensure-license-header FILE="$${platformCRD}"

	if [ -n "$(TAG)" ]; then
		echo "[INFO] Set tags in nightly OLM files"
		sed -ri "s/(.*:\s?)$(RELEASE)([^-])?$$/\1$(TAG)\2/" "$${NEW_CSV}"
	fi

	# Remove roles for kubernetes bundle
	YAML_CONTENT=$$(cat "$${NEW_CSV}")
	if [ $${platform} = "kubernetes" ]; then
		clusterPermLength=$$(echo "$${YAML_CONTENT}" | yq -r ".spec.install.spec.clusterPermissions[0].rules | length")
		i=0
		while [ "$${i}" -lt "$${clusterPermLength}" ]; do
			apiGroupLength=$$(echo "$${YAML_CONTENT}" | yq -r '.spec.install.spec.clusterPermissions[0].rules['$${i}'].apiGroups | length')
			if [ "$${apiGroupLength}" -gt 0 ]; then
				j=0
				while [ "$${j}" -lt "$${apiGroupLength}" ]; do
					apiGroup=$$(echo "$${YAML_CONTENT}" | yq -r '.spec.install.spec.clusterPermissions[0].rules['$${i}'].apiGroups['$${j}']')
					case $${apiGroup} in *openshift.io)
						# Permissions needed for DevWorkspace
						if [ "$${apiGroup}" != "route.openshift.io" ] && [ "$${apiGroup}" != oauth.openshift.io ]; then
							YAML_CONTENT=$$(echo "$${YAML_CONTENT}" | yq -rY 'del(.spec.install.spec.clusterPermissions[0].rules['$${i}'])' )
							j=$$((j-1))
							i=$$((i-1))
						fi
						break
						;;
					esac;
					j=$$((i+1))
				done
			fi
			i=$$((i+1))
		done

		permLength=$$(echo "$${YAML_CONTENT}" | yq -r ".spec.install.spec.permissions[0].rules | length")
		i=0
		while [ "$${i}" -lt "$${permLength}" ]; do
			apiGroupLength=$$(echo "$${YAML_CONTENT}" | yq -r '.spec.install.spec.permissions[0].rules['$${i}'].apiGroups | length')
			if [ "$${apiGroupLength}" -gt 0 ]; then
				j=0
				while [ "$${j}" -lt "$${apiGroupLength}" ]; do
					apiGroup=$$(echo "$${YAML_CONTENT}" | yq -r '.spec.install.spec.permissions[0].rules['$${i}'].apiGroups['$${j}']')
					case $${apiGroup} in *openshift.io)
						YAML_CONTENT=$$(echo "$${YAML_CONTENT}" | yq -rY 'del(.spec.install.spec.permissions[0].rules['$${i}'])' )
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
	fi
	echo "$${YAML_CONTENT}" > "$${NEW_CSV}"

	# Remove roles for openshift bundle
	YAML_CONTENT=$$(cat "$${NEW_CSV}")
	if [ $${platform} = "openshift" ]; then
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
	fi
	echo "$${YAML_CONTENT}" > "$${NEW_CSV}"

	if [ $${platform} = "openshift" ]; then
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
	fi

	# Fix CSV
	if [ "$${platform}" = "openshift" ]; then
		echo "[INFO] Fix openshift sample"
		sample=$$(yq -r ".metadata.annotations.\"alm-examples\"" "$${NEW_CSV}")
		fixedSample=$$(echo "$${sample}" | yq -r ".[0] | del(.spec.k8s) | [.]" | sed -r 's/"/\\"/g')
		# Update sample in the CSV
		yq -rY " (.metadata.annotations.\"alm-examples\") = \"$${fixedSample}\"" "$${NEW_CSV}" > "$${NEW_CSV}.old"
		mv "$${NEW_CSV}.old" "$${NEW_CSV}"
	fi
	if [ "$${platform}" = "kubernetes" ]; then
		echo "[INFO] Fix kubernetes sample"
		sample=$$(yq -r ".metadata.annotations.\"alm-examples\"" "$${NEW_CSV}")
		fixedSample=$$(echo "$${sample}" | yq -r ".[0] | (.spec.k8s.ingressDomain) = \"\" | del(.spec.auth.openShiftoAuth) | [.]" | sed -r 's/"/\\"/g')
		# Update sample in the CSV
		yq -rY " (.metadata.annotations.\"alm-examples\") = \"$${fixedSample}\"" "$${NEW_CSV}" > "$${NEW_CSV}.old"
		mv "$${NEW_CSV}.old" "$${NEW_CSV}"

		# Update annotations
		echo "[INFO] Update kubernetes annotations"
		yq -rYi "del(.metadata.annotations.\"operators.openshift.io/infrastructure-features\")" "$${NEW_CSV}"
	fi

	# set `app.kubernetes.io/managed-by` label
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.metadata.labels."app.kubernetes.io/managed-by") = "olm"' "$${NEW_CSV}"

	# set Pod Security Context Posture
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostIPC") = false' "$${NEW_CSV}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostNetwork") = false' "$${NEW_CSV}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostPID") = false' "$${NEW_CSV}"
	if [ "$${platform}" = "openshift" ]; then
		yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[0].securityContext."allowPrivilegeEscalation") = false' "$${NEW_CSV}"
		yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[0].securityContext."runAsNonRoot") = true' "$${NEW_CSV}"
		yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[1].securityContext."allowPrivilegeEscalation") = false' "$${NEW_CSV}"
		yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec.containers[1].securityContext."runAsNonRoot") = true' "$${NEW_CSV}"
	fi

	# Format code.
	yq -rY "." "$${NEW_CSV}" > "$${NEW_CSV}.old"
	mv "$${NEW_CSV}.old" "$${NEW_CSV}"

	# $(MAKE) ensure-license-header "$${NEW_CSV}"

getPackageName:
	if [ -z "$(platform)" ]; then
		echo "[ERROR] Please specify first argument: 'platform'"
		exit 1
	fi
	echo "eclipse-che-preview-$(platform)"

getBundlePath:
	if [ -z "$(platform)" ]; then
		echo "[ERROR] Please specify first argument: 'platform'"
		exit 1
	fi
	if [ -z "$(channel)" ]; then
		echo "[ERROR] Please specify second argument: 'channel'"
		exit 1
	fi
	PACKAGE_NAME=$$($(MAKE) getPackageName platform="$(platform)" -s)
	echo "$(PROJECT_DIR)/bundle/$(channel)/$${PACKAGE_NAME}"

increment-nightly-version:
	if [ -z "$(platform)" ]; then
		echo "[ERROR] please specify first argument 'platform'"
		exit 1
	fi

	NIGHTLY_BUNDLE_PATH=$$($(MAKE) getBundlePath platform="$(platform)" channel="nightly" -s)
	OPM_BUNDLE_MANIFESTS_DIR="$${NIGHTLY_BUNDLE_PATH}/manifests"
	CSV="$${OPM_BUNDLE_MANIFESTS_DIR}/che-operator.clusterserviceversion.yaml"

	currentNightlyVersion=$$(yq -r ".spec.version" "$${CSV}")
	echo  "[INFO] current nightly $(platform) version: $${currentNightlyVersion}"

	incrementPart=$$($(MAKE) get-nightly-version-increment nightlyVersion="$${currentNightlyVersion}" -s)

	PACKAGE_NAME="eclipse-che-preview-$(platform)"

	CLUSTER_SERVICE_VERSION=$$($(MAKE) get-current-stable-version platform="$(platform)" -s)
	STABLE_PACKAGE_VERSION=$$(echo "$${CLUSTER_SERVICE_VERSION}" | sed -e "s/$${PACKAGE_NAME}.v//")
	echo "[INFO] Current stable package version: $${STABLE_PACKAGE_VERSION}"

	# Parse stable version parts
	majorAndMinor=$${STABLE_PACKAGE_VERSION%.*}
	STABLE_MINOR_VERSION=$${majorAndMinor#*.}
	STABLE_MAJOR_VERSION=$${majorAndMinor%.*}

	STABLE_MINOR_VERSION=$$(($$STABLE_MINOR_VERSION+1))
	echo "$${STABLE_MINOR_VERSION}"

	incrementPart=$$((incrementPart+1))
	newVersion="$${STABLE_MAJOR_VERSION}.$${STABLE_MINOR_VERSION}.0-$${incrementPart}.nightly"

	echo "[INFO] Set up nightly $(platform) version: $${newVersion}"
	yq -rY "(.spec.version) = \"$${newVersion}\" | (.metadata.name) = \"eclipse-che-preview-$(platform).v$${newVersion}\"" "$${CSV}" > "$${CSV}.old"
	mv "$${CSV}.old" "$${CSV}"

get-current-stable-version:
	if [ -z "$(platform)" ]; then
		echo "[ERROR] Please specify first argument: 'platform'"
		exit 1
	fi

	STABLE_BUNDLE_PATH=$$($(MAKE) getBundlePath platform="$(platform)" channel="stable" -s)
	LAST_STABLE_CSV="$${STABLE_BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"

	lastStableVersion=$$(yq -r ".spec.version" "$${LAST_STABLE_CSV}")
	echo "$${lastStableVersion}"

get-nightly-version-increment:
	if [ -z $(nightlyVersion) ]; then
		echo "[ERROR] Provide nightly version to parse"
		exit 1
	fi

	versionWithoutNightly="$${nightlyVersion%.nightly}"

	version="$${versionWithoutNightly%-*}"

	incrementPart="$${versionWithoutNightly#*-}"

	echo "$${incrementPart}"

update-resources: SHELL := /bin/bash
update-resources: check-requirements update-resource-images update-roles
	for platform in 'kubernetes' 'openshift'
	do
		$(MAKE) bundle "platform=$${platform}"
	done

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

update-deployment-yaml-images:
	if [ -z $(UBI8_MINIMAL_IMAGE) ] || [ -z $(PLUGIN_BROKER_METADATA_IMAGE) ] || [ -z $(PLUGIN_BROKER_ARTIFACTS_IMAGE) ] || [ -z $(JWT_PROXY_IMAGE) ]; then
		echo "[ERROR] Define required arguments: `UBI8_MINIMAL_IMAGE`, `PLUGIN_BROKER_METADATA_IMAGE`, `PLUGIN_BROKER_ARTIFACTS_IMAGE`, `JWT_PROXY_IMAGE`"
		exit 1
	fi
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_pvc_jobs\") | .value ) = \"$(UBI8_MINIMAL_IMAGE)\"" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_metadata\") | .value ) = \"$(PLUGIN_BROKER_METADATA_IMAGE)\"" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_workspace_plugin_broker_artifacts\") | .value ) = \"$(PLUGIN_BROKER_ARTIFACTS_IMAGE)\"" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[] | select(.name == \"che-operator\").env[] | select(.name == \"RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image\") | .value ) = \"$(JWT_PROXY_IMAGE)\"" $(OPERATOR_YAML)
	$(MAKE) ensure-license-header FILE="config/manager/manager.yaml"

update-devworkspace-container:
	echo "[INFO] Update devworkspace container in the che-operator deployment"
	# Deletes old DWCO container
	yq -riY "del(.spec.template.spec.containers[1])" $(OPERATOR_YAML)
	yq -riY ".spec.template.spec.containers[1].name = \"devworkspace-container\"" $(OPERATOR_YAML)

	# Extract DWCO container spec from deployment
	DWCO_CONTAINER=$$(curl -sL https://raw.githubusercontent.com/che-incubator/devworkspace-che-operator/main/deploy/deployment/openshift/objects/devworkspace-che-manager.Deployment.yaml \
	| sed '1,/containers:/d' \
	| sed -n '/serviceAccountName:/q;p' \
	| sed -e 's/^/  /')
	echo "$${DWCO_CONTAINER}" > dwcontainer

	# Add DWCO container to manager.yaml
	sed -i -e '/- name: devworkspace-container/{r dwcontainer' -e 'd}' $(OPERATOR_YAML)
	rm dwcontainer

	# update securityContext
	yq -riY ".spec.template.spec.containers[1].securityContext.privileged = false" $(OPERATOR_YAML)
	yq -riY ".spec.template.spec.containers[1].securityContext.readOnlyRootFilesystem = false" $(OPERATOR_YAML)
	yq -riY ".spec.template.spec.containers[1].securityContext.capabilities.drop[0] = \"ALL\"" $(OPERATOR_YAML)

	# update env variable
	yq -riY "del( .spec.template.spec.containers[1].env[] | select(.name == \"CONTROLLER_SERVICE_ACCOUNT_NAME\") | .valueFrom)" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[1].env[] | select(.name == \"CONTROLLER_SERVICE_ACCOUNT_NAME\") | .value) = \"che-operator\"" $(OPERATOR_YAML)
	yq -riY "del( .spec.template.spec.containers[1].env[] | select(.name == \"WATCH_NAMESPACE\") | .value)" $(OPERATOR_YAML)
	yq -riY "( .spec.template.spec.containers[1].env[] | select(.name == \"WATCH_NAMESPACE\") | .valueFrom.fieldRef.fieldPath) = \"metadata.namespace\"" $(OPERATOR_YAML)

	yq -riY ".spec.template.spec.containers[1].args[1] =  \"--metrics-addr\"" $(OPERATOR_YAML)
	yq -riY ".spec.template.spec.containers[1].args[2] =  \"0\"" $(OPERATOR_YAML)

	# $(MAKE) ensureLicense $(OPERATOR_YAML)

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

	$(MAKE) update-devworkspace-container

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	if [ -z "$(platform)" ]; then
		echo "[INFO] You must specify 'platform' macros. For example: `make bundle platform=kubernetes`"
		exit 1
	fi
	BUNDLE_PACKAGE="eclipse-che-preview-$(platform)"
	BUNDLE_DIR="bundle/$(DEFAULT_CHANNEL)/$${BUNDLE_PACKAGE}"
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

chectl-templ:
	if [ -z "$(TARGET)" ];
		then echo "A";
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
