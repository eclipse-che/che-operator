#!/bin/bash
#
# Copyright (c) 2019-2023 Red Hat, Inc.
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

ifeq (,$(shell which kubectl)$(shell which oc))
	$(error oc or kubectl is required to proceed)
endif

ifneq (,$(shell which kubectl))
	K8S_CLI := kubectl
else
	K8S_CLI := oc
endif

# Detect image tool
ifeq ($(IMAGE_TOOL),)
ifneq (,$(shell which docker))
	IMAGE_TOOL := docker
else
	IMAGE_TOOL := podman
endif
endif

ifndef VERBOSE
	MAKEFLAGS += --silent
endif

ifeq ($(shell $(K8S_CLI) api-resources --api-group='route.openshift.io' 2>&1 | grep -o routes),routes)
  PLATFORM := openshift
else
  PLATFORM := kubernetes
endif

PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
echo "[INFO] Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
}
endef

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir := $(dir $(mkfile_path))

# Default Eclipse Che operator image
IMG ?= quay.io/eclipse/che-operator:next

CONFIG_MANAGER="config/manager/manager.yaml"

INTERNAL_TMP_DIR=/tmp/che-operator-dev
BASH_ENV_FILE=$(INTERNAL_TMP_DIR)/bash.env
VSCODE_ENV_FILE=$(INTERNAL_TMP_DIR)/vscode.env

DEPLOYMENT_DIR=$(PROJECT_DIR)/deploy/deployment

ifneq (,$(shell $(K8S_CLI) get checluster -A 2>/dev/null))
  ECLIPSE_CHE_NAMESPACE := $(shell $(K8S_CLI) get checluster -A -o "jsonpath={.items[0].metadata.namespace}")
else
  ECLIPSE_CHE_NAMESPACE ?= "eclipse-che"
endif

ifneq (,$(shell $(K8S_CLI) get pod -l app.kubernetes.io/component=che-operator -A 2>/dev/null))
  OPERATOR_NAMESPACE := $(shell $(K8S_CLI) get pod -l app.kubernetes.io/component=che-operator -A -o "jsonpath={.items[0].metadata.namespace}")
else
  OPERATOR_NAMESPACE ?= "eclipse-che"
endif

ECLIPSE_CHE_PACKAGE_NAME=eclipse-che

CHECLUSTER_CR_PATH="$(PROJECT_DIR)/config/samples/org_v2_checluster.yaml"
CHECLUSTER_CRD_PATH="$(PROJECT_DIR)/config/crd/bases/org.eclipse.che_checlusters.yaml"

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
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

build: generate ## Build Eclipse Che operator binary
	go build -o bin/manager main.go

run: SHELL := /bin/bash
run: install-che-operands genenerate-env  ## Run Eclipse Che operator
	source $(BASH_ENV_FILE)
	go run ./main.go

debug: SHELL := /bin/bash
debug: install-che-operands genenerate-env ## Run and debug Eclipse Che operator
	source $(BASH_ENV_FILE)
	# dlv has an issue with 'Ctrl-C' termination, that's why we're doing trick with detach.
	dlv debug --listen=:2345 --headless=true --api-version=2 ./main.go -- &
	DLV_PID=$!
	wait $${DLV_PID}

docker-build: ## Build Eclipse Che operator image
	if [ "$(SKIP_TESTS)" = true ]; then
		${IMAGE_TOOL} build -t ${IMG} --build-arg SKIP_TESTS=true .
	else
		${IMAGE_TOOL} build -t ${IMG} .
	fi

docker-push: ## Push Eclipse Che operator image to a registry
	${IMAGE_TOOL} push ${IMG}

update-dev-resources: SHELL := /bin/bash
update-dev-resources: validate-requirements ## Update development resources
	# Update ubi8 image
	ubiMinimal8Version=$$(skopeo --override-os linux inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.version')
	ubiMinimal8Release=$$(skopeo --override-os linux inspect docker://registry.access.redhat.com/ubi8-minimal:latest | jq -r '.Labels.release')
	UBI8_MINIMAL_IMAGE="registry.access.redhat.com/ubi8-minimal:$${ubiMinimal8Version}-$${ubiMinimal8Release}"
	skopeo --override-os linux inspect docker://$${UBI8_MINIMAL_IMAGE} > /dev/null
	echo "[INFO] UBI8 image $${UBI8_MINIMAL_IMAGE}"

	# Dockerfile
	sed -i 's|registry.access.redhat.com/ubi8-minimal:[^\s]* |'$${UBI8_MINIMAL_IMAGE}' |g' $(PROJECT_DIR)/Dockerfile

	$(MAKE) bundle CHANNEL=next
	$(MAKE) gen-deployment
	$(MAKE) update-helmcharts CHANNEL=next
	$(MAKE) fmt

gen-deployment: SHELL := /bin/bash
gen-deployment: manifests download-kustomize kustomize-operator-image ## Generate Eclipse Che k8s deployment resources
	rm -rf $(DEPLOYMENT_DIR)
	for TARGET_PLATFORM in kubernetes openshift; do
		PLATFORM_DIR=$(DEPLOYMENT_DIR)/$${TARGET_PLATFORM}
		OBJECTS_DIR=$${PLATFORM_DIR}/objects

		mkdir -p $${OBJECTS_DIR}

		COMBINED_FILENAME=$${PLATFORM_DIR}/combined.yaml
		$(KUSTOMIZE) build config/$${TARGET_PLATFORM} | cat > $${COMBINED_FILENAME} -

		# Split the giant files output by kustomize per-object
		csplit -s -f "temp" --suppress-matched "$${COMBINED_FILENAME}" '/^---$$/' '{*}'
		for file in temp??; do
			name_kind=$$(yq -r '"\(.metadata.name).\(.kind)"' "$${file}")
			mv "$${file}" "$${OBJECTS_DIR}/$${name_kind}.yaml"
		done
		cp $(PROJECT_DIR)/config/samples/org_v2_checluster.yaml $${PLATFORM_DIR}

		echo "[INFO] Deployments resources generated into $${PLATFORM_DIR}"
	done

update-helmcharts: SHELL := /bin/bash
update-helmcharts: ## Update Helm Charts based on deployment resources
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }

	HELM_DIR=$(PROJECT_DIR)/helmcharts/$(CHANNEL)
	HELMCHARTS_TEMPLATES=$${HELM_DIR}/templates
	HELMCHARTS_CRDS=$${HELM_DIR}/crds

	rm -rf $${HELMCHARTS_TEMPLATES} $${HELMCHARTS_CRDS}
	mkdir -p $${HELMCHARTS_TEMPLATES} $${HELMCHARTS_CRDS}

	rsync -a --exclude='checlusters.org.eclipse.che.CustomResourceDefinition.yaml' $(DEPLOYMENT_DIR)/kubernetes/objects/ $${HELMCHARTS_TEMPLATES}
	cp $(DEPLOYMENT_DIR)/kubernetes/org_v2_checluster.yaml $${HELMCHARTS_TEMPLATES}
	cp $(DEPLOYMENT_DIR)/kubernetes/objects/checlusters.org.eclipse.che.CustomResourceDefinition.yaml $${HELMCHARTS_CRDS}

	# Remove namespace since its creation is mentioned is README.md
	rm $${HELMCHARTS_TEMPLATES}/eclipse-che.Namespace.yaml

	if [ $(CHANNEL) == "stable" ]; then
		chartYaml=$${HELM_DIR}/Chart.yaml

		CRDS_SAMPLES_FILES=(
			$${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
		)

		CRDS_SAMPLES=""
		for CRD_SAMPLE in "$${CRDS_SAMPLES_FILES[@]}"; do
			CRD_SAMPLE=$$(cat $${CRD_SAMPLE} | yq -rY ". | (.metadata.namespace = \"$(ECLIPSE_CHE_NAMESPACE)\") | [.]")
		 	CRDS_SAMPLES=$${CRDS_SAMPLES}$${CRD_SAMPLE}$$'\n'
		done

		# Update Chart.yaml
		yq -rYi --arg examples "$${CRDS_SAMPLES}" ".annotations.\"artifacthub.io/crdsExamples\" = \$$examples" $${chartYaml}

		# Set CheCluster API version to v2 (TODO: remove in a next release)
		CRDS=$$(yq -r '.annotations."artifacthub.io/crds"' $${chartYaml} | yq -ry -w 9999 '.[0].version="v2"')
		yq -rYi --arg crds "$${CRDS}" ".annotations.\"artifacthub.io/crds\" = \$$crds" $${chartYaml}
		sed -i 's|org_v1_checluster|org_v2_checluster|g' $${HELM_DIR}/README.md

		make license $${chartYaml}

		rm -rf $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
	else
		yq -riY '.spec.networking = null' $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
		yq -riY '.spec.networking.tlsSecretName = "che-tls"' $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
		yq -riY '.spec.networking.domain = "{{ .Values.networking.domain }}"' $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
		yq -riY '.spec.networking.auth.oAuthSecret = "{{ .Values.networking.auth.oAuthSecret }}"' $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
		yq -riY '.spec.networking.auth.oAuthClientName = "{{ .Values.networking.auth.oAuthClientName }}"' $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
		yq -riY '.spec.networking.auth.identityProviderURL = "{{ .Values.networking.auth.identityProviderURL }}"' $${HELMCHARTS_TEMPLATES}/org_v2_checluster.yaml
	fi

	echo "[INFO] HelmCharts updated $${HELM_DIR}"

gen-chectl-tmpl: SHELL := /bin/bash
gen-chectl-tmpl: ## Generate Eclipse Che k8s deployment resources used by chectl
	[[ -z "$(TEMPLATES)" ]] && { echo [ERROR] TEMPLATES not defined; exit 1; }
	[[ -z "$(SOURCE_PROJECT)" ]] && src=$(PROJECT_DIR) || src=$(SOURCE_PROJECT)
	[[ -z "$(DWO_VERSION)" ]] && DEFINED_DWO_VERSION="main" || DEFINED_DWO_VERSION=$(DWO_VERSION)

	cheOperatorDst="$(TEMPLATES)/che-operator/kubernetes" && rm -rf $${cheOperatorDst}
	devWorkspaceOperatorDst="$(TEMPLATES)/devworkspace-operator/kubernetes" && rm -rf $${devWorkspaceOperatorDst}

	src="$${src}/deploy/deployment/kubernetes/objects"
	mkdir -p "$${cheOperatorDst}/crds" "$${devWorkspaceOperatorDst}"

	cp $${src}/che-operator.Deployment.yaml $${cheOperatorDst}/operator.yaml
	cp $${src}/checlusters.org.eclipse.che.CustomResourceDefinition.yaml $${cheOperatorDst}/crds/org.eclipse.che_checlusters.yaml
	cp $${src}/../org_v2_checluster.yaml $${cheOperatorDst}/crds/org_checluster_cr.yaml
	cp $${src}/che-operator.ServiceAccount.yaml $${cheOperatorDst}/service_account.yaml
	cp $${src}/che-operator.ClusterRole.yaml $${cheOperatorDst}/cluster_role.yaml
	cp $${src}/che-operator.ClusterRoleBinding.yaml $${cheOperatorDst}/cluster_rolebinding.yaml
	cp $${src}/che-operator.Role.yaml $${cheOperatorDst}/role.yaml
	cp $${src}/che-operator.RoleBinding.yaml $${cheOperatorDst}/role_binding.yaml
	if [[ -f $${src}/che-operator-leader-election.Role.yaml ]]; then
		cp $${src}/che-operator-leader-election.Role.yaml $${cheOperatorDst}/leader-election-role.yaml
	fi
	if [[ -f $${src}/che-operator-leader-election.RoleBinding.yaml ]]; then
		cp $${src}/che-operator-leader-election.RoleBinding.yaml $${cheOperatorDst}/leader-election-cluster_rolebinding.yaml
	fi
	cp $${src}/che-operator-service.Service.yaml $${cheOperatorDst}/webhook-service.yaml
	if [[ -f $${src}/org.eclipse.che.ValidatingWebhookConfiguration.yaml ]]; then
	  cp $${src}/org.eclipse.che.ValidatingWebhookConfiguration.yaml $${cheOperatorDst}/org.eclipse.che.ValidatingWebhookConfiguration.yaml
	fi
	if [[ -f $${src}/org.eclipse.che.MutatingWebhookConfiguration.yaml ]]; then
	  cp $${src}/org.eclipse.che.MutatingWebhookConfiguration.yaml $${cheOperatorDst}/org.eclipse.che.MutatingWebhookConfiguration.yaml
	fi
	cp $${src}/che-operator-serving-cert.Certificate.yaml $${cheOperatorDst}/serving-cert.yaml
	cp $${src}/che-operator-selfsigned-issuer.Issuer.yaml $${cheOperatorDst}/selfsigned-issuer.yaml

	curl -s https://raw.githubusercontent.com/devfile/devworkspace-operator/$${DEFINED_DWO_VERSION}/deploy/deployment/kubernetes/combined.yaml -o $${devWorkspaceOperatorDst}/combined.yaml

	echo "[INFO] Generated chectl templates into $(TEMPLATES)"

manifests: download-controller-gen download-addlicense ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=manager-role paths="./..." output:crd:artifacts:config=config/crd/bases

	# remove yaml delimitier, which makes OLM catalog source image broken.
	sed -i '/---/d' "$(CHECLUSTER_CRD_PATH)"

	$(MAKE) license $$(find ./config/crd -not -path "./vendor/*" -name "*.yaml")

generate: download-controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: download-addlicense ## Run go fmt against code.
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

	$(MAKE) license $${FILES_TO_CHECK_LICENSE}

vet: ## Run go vet against code.
	go vet ./...

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: download-gateway-resources ## Run tests.
	export MOCK_API=true; go test -mod=vendor ./... -coverprofile cover.out

update-go-dependencies:  ## Update golang dependencies
	go mod tidy
	go mod vendor

license: ## Add license to the files
	FILES=$$(echo $(filter-out $@,$(MAKECMDGOALS)))
	$(ADD_LICENSE) -f hack/license-header.txt $${FILES}

# Generates environment files used by bash and vscode
genenerate-env:
	mkdir -p $(INTERNAL_TMP_DIR)
	cat $(CONFIG_MANAGER) \
	  | yq -r \
	    '.spec.template.spec.containers[]
	      | select(.name=="che-operator")
	      | .env[]
	      | select(has("value"))
	      | "export \(.name)=\"\(.value)\""' \
	  > $(BASH_ENV_FILE)
	echo "export WATCH_NAMESPACE=$(ECLIPSE_CHE_NAMESPACE)" >> $(BASH_ENV_FILE)
	echo "[INFO] Created $(BASH_ENV_FILE)"

	cat $(CONFIG_MANAGER) \
	  | yq -r \
	    '.spec.template.spec.containers[]
	      | select(.name=="che-operator")
	      | .env[]
	      | select(has("value"))
	      | "\(.name)=\"\(.value)\""' \
	  > $(VSCODE_ENV_FILE)
	echo "WATCH_NAMESPACE=$(ECLIPSE_CHE_NAMESPACE)" >> $(VSCODE_ENV_FILE)
	echo "[INFO] Created $(VSCODE_ENV_FILE)"

	cat $(BASH_ENV_FILE)

install-che-operands: SHELL := /bin/bash
install-che-operands: generate manifests download-kustomize download-gateway-resources
	echo "[INFO] Running on $(PLATFORM)"
	if [[ ! "$$($(K8S_CLI) get checluster eclipse-che -n $(ECLIPSE_CHE_NAMESPACE) || false )" ]]; then
		[[ $(PLATFORM) == "kubernetes" ]] && $(MAKE) install-certmgr
		$(MAKE) install-devworkspace CHANNEL="next"
		$(KUSTOMIZE) build config/$(PLATFORM) | $(K8S_CLI) apply -f -
		$(MAKE) wait-pod-running SELECTOR="app.kubernetes.io/component=che-operator" NAMESPACE=$(ECLIPSE_CHE_NAMESPACE)
	fi

	$(K8S_CLI) scale deploy che-operator -n $(OPERATOR_NAMESPACE) --replicas=0

	# Disable Webhooks since che operator pod is scaled down
	$(K8S_CLI) delete validatingwebhookconfiguration org.eclipse.che
	$(K8S_CLI) delete mutatingwebhookconfiguration org.eclipse.che
	$(K8S_CLI) patch crd checlusters.org.eclipse.che --patch '{"spec": {"conversion": null}}' --type=merge

	$(MAKE) store_tls_cert
	$(MAKE) create-checluster-cr

# Downloads Gateway resources
download-gateway-resources:
	GATEWAY_RESOURCES=/tmp/header-rewrite-traefik-plugin

	rm -rf /tmp/asset-header-rewrite-traefik-plugin.zip /tmp/*-header-rewrite-traefik-plugin-*/ $${GATEWAY_RESOURCES}
	mkdir -p $${GATEWAY_RESOURCES}
	curl -sL https://api.github.com/repos/che-incubator/header-rewrite-traefik-plugin/zipball/${DEV_HEADER_REWRITE_TRAEFIK_PLUGIN} > /tmp/asset-header-rewrite-traefik-plugin.zip
	unzip -q /tmp/asset-header-rewrite-traefik-plugin.zip -d /tmp
	mv /tmp/*-header-rewrite-traefik-plugin-*/headerRewrite.go /tmp/*-header-rewrite-traefik-plugin-*/.traefik.yml $${GATEWAY_RESOURCES}

	echo "[INFO] Gateway resources downloaded into  $${GATEWAY_RESOURCES}"

# Store `che-operator-webhook-server-cert` secret locally
store_tls_cert:
	mkdir -p /tmp/k8s-webhook-server/serving-certs/
	$(K8S_CLI) get secret che-operator-service-cert -n $(OPERATOR_NAMESPACE) -o json | jq -r '.data["tls.crt"]' | base64 -d > /tmp/k8s-webhook-server/serving-certs/tls.crt
	$(K8S_CLI) get secret che-operator-service-cert -n $(OPERATOR_NAMESPACE) -o json | jq -r '.data["tls.key"]' | base64 -d > /tmp/k8s-webhook-server/serving-certs/tls.key

##@ OLM catalog

.PHONY: bundle
bundle: SHELL := /bin/bash
bundle: generate manifests download-kustomize download-operator-sdk ## Generate OLM bundle
	echo "[INFO] Updating OperatorHub bundle"

	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	[[ "$(INCREMENT_BUNDLE_VERSION)" == false ]] || $(MAKE) increment-bundle-version

	BUNDLE_PATH=$$($(MAKE) bundle-path)
	CSV_PATH=$$($(MAKE) csv-path)
	NEXT_BUNDLE_VERSION=$$($(MAKE) bundle-version)
	NEXT_BUNDLE_CREATION_DATE=$$(yq -r ".metadata.annotations.createdAt" "$${CSV_PATH}")

	# Build default clusterserviceversion file
	$(OPERATOR_SDK) generate kustomize manifests

	$(KUSTOMIZE) build config/openshift/olm | \
	$(OPERATOR_SDK) generate bundle \
	--quiet \
	--overwrite \
	--version $${NEXT_BUNDLE_VERSION} \
	--package $(ECLIPSE_CHE_PACKAGE_NAME) \
	--output-dir $${BUNDLE_PATH} \
	--channels $(CHANNEL) \
	--default-channel $(CHANNEL)

	# Rename clusterserviceversion file
	mv $${BUNDLE_PATH}/manifests/$(ECLIPSE_CHE_PACKAGE_NAME).clusterserviceversion.yaml $${CSV_PATH}

	# Rollback creation date if version is not incremented
	if [[ "$(INCREMENT_BUNDLE_VERSION)" == false ]]; then
		sed -i "s/createdAt:.*$$/createdAt: \"$${NEXT_BUNDLE_CREATION_DATE}\"/" "$${CSV_PATH}"
	fi

	# Copy bundle.Dockerfile to the bundle dir
 	# Update paths (since it is created in the root of the project) and labels
	mv bundle.Dockerfile $${BUNDLE_PATH}
	sed -i 's|$(PROJECT_DIR)/bundle/$(CHANNEL)/$(ECLIPSE_CHE_PACKAGE_NAME)/||' $${BUNDLE_PATH}/bundle.Dockerfile
	printf "\nLABEL com.redhat.openshift.versions=\"v4.8\"" >> $${BUNDLE_PATH}/bundle.Dockerfile

	# Update annotations.yaml correspondingly to bundle.Dockerfile
	printf "\n  com.redhat.openshift.versions: \"v4.8\"" >> $${BUNDLE_PATH}/metadata/annotations.yaml

	# Base cluster service version file has got correctly sorted CRDs.
	# They are sorted with help of annotation markers in the api type files ("api/v1" folder).
	# Example such annotation: +operator-sdk:csv:customresourcedefinitions:order=0
	# Copy this sorted CRDs to the bundle clusterserviceversion file.
	CRDS_OWNED=$$(yq  '.spec.customresourcedefinitions.owned' "$(PROJECT_DIR)/config/manifests/bases/che-operator.clusterserviceversion.yaml")
	yq -riSY ".spec.customresourcedefinitions.owned = $${CRDS_OWNED}" "$${CSV_PATH}"

	# Kustomize won't set default values
	# Update deployment explicitly
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostIPC") = false' "$${CSV_PATH}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostNetwork") = false' "$${CSV_PATH}"
	yq -riSY  '(.spec.install.spec.deployments[0].spec.template.spec."hostPID") = false' "$${CSV_PATH}"

	# Fix examples by removing some special characters
	FIXED_ALM_EXAMPLES=$$(yq -r '.metadata.annotations["alm-examples"]' $${CSV_PATH}  | sed -r 's/"/\\"/g')
	yq -riY ".metadata.annotations[\"alm-examples\"] = \"$${FIXED_ALM_EXAMPLES}\"" $${CSV_PATH}

	# Update image
	yq -riY '.metadata.annotations.containerImage = "'$(IMG)'"' $${CSV_PATH}
	yq -riY '.spec.install.spec.deployments[0].spec.template.spec.containers[0].image = "'$(IMG)'"' $${CSV_PATH}

	# Format file
	yq -riY "." "$${BUNDLE_PATH}/manifests/org.eclipse.che_checlusters.yaml"

	$(MAKE) license $$(find $${BUNDLE_PATH} -name "*.yaml")

	$(OPERATOR_SDK) bundle validate $${BUNDLE_PATH} --select-optional name=operatorhub --optional-values=k8s-version=1.19

bundle-build: SHELL := /bin/bash
bundle-build: download-opm ## Build a bundle image
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	[[ -z "$(BUNDLE_IMG)" ]] && { echo [ERROR] BUNDLE_IMG not defined; exit 1; }

	BUNDLE_PATH=$$($(MAKE) bundle-path)
	$(IMAGE_TOOL) build -f $${BUNDLE_PATH}/bundle.Dockerfile -t $(BUNDLE_IMG) $${BUNDLE_PATH}

bundle-push: SHELL := /bin/bash
bundle-push: ## Push a bundle image
	[[ -z "$(BUNDLE_IMG)" ]] && { echo [ERROR] BUNDLE_IMG not defined; exit 1; }
	$(IMAGE_TOOL) push $(BUNDLE_IMG)

bundle-render: SHELL := /bin/bash
bundle-render: download-opm ## Add bundle to a catalog
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	[[ -z "$(BUNDLE_NAME)" ]] && { echo [ERROR] BUNDLE_NAME not defined; exit 1; }
	[[ -z "$(BUNDLE_IMG)" ]] && { echo [ERROR] BUNDLE_IMG not defined; exit 1; }
	[[ -z "$(CATALOG_DIR)" ]] && DEFINED_CATALOG_DIR=$$($(MAKE) catalog-path) || DEFINED_CATALOG_DIR=$(CATALOG_DIR)

	$(OPM) render $(BUNDLE_IMG) -o yaml --skip-tls-verify | sed 's|---||g' > $${DEFINED_CATALOG_DIR}/$(BUNDLE_NAME).bundle.yaml

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
catalog-build: SHELL := /bin/bash
catalog-build: download-opm ## Build a catalog image
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	[[ -z "$(CATALOG_IMG)" ]] && { echo [ERROR] CATALOG_IMG not defined; exit 1; }

	$(OPM) validate olm-catalog/$(CHANNEL)
	$(IMAGE_TOOL) build -f olm-catalog/index.Dockerfile -t $(CATALOG_IMG) --build-arg CHANNEL=$(CHANNEL) .

catalog-push: SHELL := /bin/bash
catalog-push: ## Push a catalog image
	[[ -z "$(CATALOG_IMG)" ]] && { echo [ERROR] CATALOG_IMG not defined; exit 1; }
	$(IMAGE_TOOL) push $(CATALOG_IMG)

bundle-path: SHELL := /bin/bash
bundle-path: ## Prints path to a bundle directory for a given channel
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	echo "$(PROJECT_DIR)/bundle/$(CHANNEL)/$(ECLIPSE_CHE_PACKAGE_NAME)"

catalog-path: SHELL := /bin/bash
catalog-path: ## Prints path to a catalog directory for a given channel
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	echo "$(PROJECT_DIR)/olm-catalog/$(CHANNEL)"

channel-path: SHELL := /bin/bash
channel-path: ## Prints path to a channel.yaml
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	echo "$(PROJECT_DIR)/olm-catalog/$(CHANNEL)/channel.yaml"

csv-path: SHELL := /bin/bash
csv-path: ## Prints path to a clusterserviceversion file for a given channel
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	BUNDLE_PATH=$$($(MAKE) bundle-path)
	echo "$${BUNDLE_PATH}/manifests/che-operator.clusterserviceversion.yaml"

bundle-name: SHELL := /bin/bash
bundle-name: ## Prints a clusterserviceversion name for a given channel
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	CSV_PATH=$$($(MAKE) csv-path)
	echo $$(yq -r ".metadata.name" "$${CSV_PATH}")

bundle-version: SHELL := /bin/bash
bundle-version: ## Prints a bundle version for a given channel
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	CSV_PATH=$$($(MAKE) csv-path)
	echo $$(yq -r ".spec.version" "$${CSV_PATH}")

##@ Utilities

OPM ?= $(shell pwd)/bin/opm
download-opm: SHELL := /bin/bash
download-opm: ## Download opm tool
	[[ -z "$(DEST)" ]] && dest=$(OPM) || dest=$(DEST)/opm
	command -v $(OPM) >/dev/null 2>&1 && exit

	OS=$(shell go env GOOS)
	ARCH=$(shell go env GOARCH)
	OPM_VERSION=$$(yq -r '.opm' $(PROJECT_DIR)/REQUIREMENTS)

	echo "[INFO] Downloading opm version: $${OPM_VERSION}"

	mkdir -p $$(dirname "$${dest}")
	curl -sL https://github.com/operator-framework/operator-registry/releases/download/$${OPM_VERSION}/$${OS}-$${ARCH}-opm > $${dest}
	chmod +x $${dest}

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
download-controller-gen: ## Download controller-gen tool
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
download-kustomize: ## Download kustomize tool
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.7)

ADD_LICENSE = $(shell pwd)/bin/addlicense
download-addlicense: ## Download addlicense tool
	$(call go-get-tool,$(ADD_LICENSE),github.com/google/addlicense@99ebc9c9db7bceb8623073e894533b978d7b7c8a)

OPERATOR_SDK ?= $(shell pwd)/bin/operator-sdk
download-operator-sdk: SHELL := /bin/bash
download-operator-sdk: ## Downloads operator sdk tool
	[[ -z "$(DEST)" ]] && dest=$(OPERATOR_SDK) || dest=$(DEST)/operator-sdk
	command -v $${dest} >/dev/null 2>&1 && exit

	OS=$(shell go env GOOS)
	ARCH=$(shell go env GOARCH)
	OPERATOR_SDK_VERSION=$$(yq -r '."operator-sdk"' $(PROJECT_DIR)/REQUIREMENTS)

	echo "[INFO] Downloading operator-sdk version $${OPERATOR_SDK_VERSION} into $${dest}"
	mkdir -p $$(dirname "$${dest}")
	curl -sL https://github.com/operator-framework/operator-sdk/releases/download/$${OPERATOR_SDK_VERSION}/operator-sdk_$${OS}_$${ARCH} > $${dest}
	chmod +x $${dest}

# Check if all required packages are installed
validate-requirements: SHELL := /bin/bash
validate-requirements:
	command -v yq >/dev/null 2>&1 || { echo "[ERROR] yq is not installed. See https://github.com/kislyuk/yq"; exit 1; }
	command -v skopeo >/dev/null 2>&1 || { echo "[ERROR] skopeo is not installed."; exit 1; }

# Set a new operator image for kustomize
kustomize-operator-image: download-kustomize
	cd config/manager
	$(KUSTOMIZE) edit set image quay.io/eclipse/che-operator:next=$(IMG)
	cd ../..

# Set a new version for the next channel
increment-bundle-version: SHELL := /bin/bash
increment-bundle-version:
	echo "[INFO] Increment bundle version for the next channel"

	STABLE_BUNDLE_VERSION=$$($(MAKE) bundle-version CHANNEL=stable)
	echo "[INFO] Current stable version: $${STABLE_BUNDLE_VERSION}"

	# Parse stable bundle version
	STABLE_BUNDLE_MAJOR_AND_MINOR_VERSION=$${STABLE_BUNDLE_VERSION%.*}
	STABLE_BUNDLE_MINOR_VERSION=$${STABLE_BUNDLE_MAJOR_AND_MINOR_VERSION#*.}
	STABLE_BUNDLE_MAJOR_VERSION=$${STABLE_BUNDLE_MAJOR_AND_MINOR_VERSION%.*}

	NEXT_BUNDLE_VERSION=$$($(MAKE) bundle-version CHANNEL=next)
	echo "[INFO] Current next version: $${NEXT_BUNDLE_VERSION}"

	# Parse next bundle version
	NEXT_BUNDLE_VERSION_STIPPED_NEXT="$${NEXT_BUNDLE_VERSION%.next*}"
	NEXT_BUNDLE_VERSION_INCRIMENT_PART="$${NEXT_BUNDLE_VERSION_STIPPED_NEXT#*-}"

	# Set a new next bundle version
	NEW_NEXT_BUNDLE_VERSION="$${STABLE_BUNDLE_MAJOR_VERSION}.$$(($$STABLE_BUNDLE_MINOR_VERSION+1)).0-$$(($$NEXT_BUNDLE_VERSION_INCRIMENT_PART+1)).next"

	# Update csv
	NEXT_CSV_PATH=$$($(MAKE) csv-path CHANNEL=next)
	yq -riY "(.spec.version) = \"$${NEW_NEXT_BUNDLE_VERSION}\" | (.metadata.name) = \"$(ECLIPSE_CHE_PACKAGE_NAME).v$${NEW_NEXT_BUNDLE_VERSION}\"" $${NEXT_CSV_PATH}

	echo "[INFO] New next version: $${NEW_NEXT_BUNDLE_VERSION}"


##@ Kubernetes helper

create-catalogsource: SHELL := /bin/bash
create-catalogsource: ## Creates catalog source
	[[ -z "$(NAME)" ]] && { echo [ERROR] NAME not defined; exit 1; }
	[[ -z "$(IMAGE)" ]] && { echo [ERROR] IMAGE not defined; exit 1; }
	[[ -z "$(NAMESPACE)" ]] && DEFINED_NAMESPACE="openshift-marketplace" || DEFINED_NAMESPACE=$(NAMESPACE)

	echo '{
	  "apiVersion": "operators.coreos.com/v1alpha1",
	  "kind": "CatalogSource",
	  "metadata": {
		"name": "$(NAME)",
		"namespace": "'$${DEFINED_NAMESPACE}'",
		"labels": {
		  "app.kubernetes.io/part-of": "che.eclipse.org"
		}
	  },
	  "spec": {
		"sourceType": "grpc",
		"publisher": "$(PUBLISHER)",
		"displayName": "$(DISPLAY_NAME)",
		"image": "$(IMAGE)",
		"updateStrategy": {
		  "registryPoll": {
			"interval": "15m"
		  }
		}
	  }
	}' | $(K8S_CLI) apply -f -

	sleep 20s
	$(K8S_CLI) wait --for=condition=ready pod -l "olm.catalogSource=$(NAME)" -n $${DEFINED_NAMESPACE} --timeout=240s

create-operatorgroup: SHELL := /bin/bash
create-operatorgroup: ## Creates operator group
	[[ -z "$(NAME)" ]] && { echo [ERROR] NAME not defined; exit 1; }
	[[ -z "$(NAMESPACE)" ]] && { echo [ERROR] NAMESPACE not defined; exit 1; }

	if [[ $$($(K8S_CLI) get operatorgroup -n "$(NAMESPACE)" --no-headers | wc -l) == 0 ]]; then
		echo '{
			"apiVersion": "operators.coreos.com/v1",
			"kind": "OperatorGroup",
			"metadata": {
			  "name": "$(NAME)",
			  "namespace": "$(NAMESPACE)"
			},
			"spec": {}
		  }' | $(K8S_CLI) apply -f -
	fi

create-subscription: SHELL := /bin/bash
create-subscription: ## Creates subscription
	[[ -z "$(NAME)" ]] && { echo [ERROR] NAME not defined; exit 1; }
	[[ -z "$(NAMESPACE)" ]] && { echo [ERROR] NAMESPACE not defined; exit 1; }
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	[[ -z "$(INSTALL_PLAN_APPROVAL)" ]] && { echo [ERROR] INSTALL_PLAN_APPROVAL not defined; exit 1; }
	[[ -z "$(PACKAGE_NAME)" ]] && { echo [ERROR] PACKAGE_NAME not defined; exit 1; }
	[[ -z "$(SOURCE)" ]] && { echo [ERROR] SOURCE not defined; exit 1; }
	[[ -z "$(SOURCE_NAMESPACE)" ]] && { echo [ERROR] SOURCE_NAMESPACE not defined; exit 1; }

	echo '{
		"apiVersion": "operators.coreos.com/v1alpha1",
		"kind": "Subscription",
		"metadata": {
		  "name": "$(NAME)",
		  "namespace": "$(NAMESPACE)"
		},
		"spec": {
		  "channel": "$(CHANNEL)",
		  "installPlanApproval": "$(INSTALL_PLAN_APPROVAL)",
		  "name": "$(PACKAGE_NAME)",
		  "source": "$(SOURCE)",
		  "sourceNamespace": "$(SOURCE_NAMESPACE)",
		  "startingCSV": "$(STARTING_CSV)"
		}
	  }' | $(K8S_CLI) apply -f -

	if [[ $(INSTALL_PLAN_APPROVAL) == "Manual" ]]; then
		$(K8S_CLI) wait subscription $(NAME) -n $(NAMESPACE) --for=condition=InstallPlanPending --timeout=60s
	fi

approve-installplan: SHELL := /bin/bash
approve-installplan: ## Approves install plan
	[[ -z "$(SUBSCRIPTION_NAME)" ]] && { echo [ERROR] SUBSCRIPTION_NAME not defined; exit 1; }
	[[ -z "$(NAMESPACE)" ]] && { echo [ERROR] NAMESPACE not defined; exit 1; }

	INSTALL_PLAN_NAME=$$($(K8S_CLI) get subscription $(SUBSCRIPTION_NAME) -n $(NAMESPACE) -o jsonpath='{.status.installplan.name}')
	$(K8S_CLI) patch installplan $${INSTALL_PLAN_NAME} -n $(NAMESPACE) --type=merge -p '{"spec":{"approved":true}}'
	$(K8S_CLI) wait installplan $${INSTALL_PLAN_NAME} -n $(NAMESPACE) --for=condition=Installed --timeout=240s

create-namespace: SHELL := /bin/bash
create-namespace: ## Creates namespace
	[[ -z "$(NAMESPACE)" ]] && DEFINED_NAMESPACE=$(ECLIPSE_CHE_NAMESPACE) || DEFINED_NAMESPACE=$(NAMESPACE)
	$(K8S_CLI) create namespace $${DEFINED_NAMESPACE} || true

wait-pod-running: SHELL := /bin/bash
wait-pod-running: ## Wait until pod is up and running
	[[ -z "$(SELECTOR)" ]] && { echo [ERROR] SELECTOR not defined; exit 1; }
	[[ -z "$(NAMESPACE)" ]] && { echo [ERROR] NAMESPACE not defined; exit 1; }

	while [ $$($(K8S_CLI) get pod -l $(SELECTOR) -n $(NAMESPACE) -o go-template='{{len .items}}') -eq 0 ]; do
		sleep 10s
	done
	$(K8S_CLI) wait --for=condition=ready pod -l $(SELECTOR) -n $(NAMESPACE) --timeout=120s

install-certmgr: SHELL := /bin/bash
install-certmgr: ## Install Cert Manager v1.7.1
	$(K8S_CLI) apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/cert-manager.yaml
	$(MAKE) wait-pod-running SELECTOR="app.kubernetes.io/component=controller" NAMESPACE=cert-manager
	$(MAKE) wait-pod-running SELECTOR="app.kubernetes.io/component=controller" NAMESPACE=cert-manager
	$(MAKE) wait-pod-running SELECTOR="app.kubernetes.io/component=webhook" NAMESPACE=cert-manager

install-devworkspace: SHELL := /bin/bash
install-devworkspace: ## Install Dev Workspace operator, available channels: next, fast
	[[ -z "$(CHANNEL)" ]] && { echo [ERROR] CHANNEL not defined; exit 1; }
	[[ -z "$(OPERATOR_NAMESPACE)" ]] && DEFINED_OPERATOR_NAMESPACE="openshift-operators" || DEFINED_OPERATOR_NAMESPACE=$(OPERATOR_NAMESPACE)

	if [[ $(PLATFORM) == "kubernetes" ]]; then
		$(MAKE) create-namespace NAMESPACE="devworkspace-controller"
		if [[ $(CHANNEL) == "fast" ]]; then
			rm -rf /tmp/dwo
			git clone --quiet https://github.com/devfile/devworkspace-operator /tmp/dwo
			pushd /tmp/dwo
			DWO_STABLE_VERSION=$$(git describe --tags $$(git rev-list --tags --max-count=1))
			popd

			$(K8S_CLI) apply -f https://raw.githubusercontent.com/devfile/devworkspace-operator/$${DWO_STABLE_VERSION}/deploy/deployment/kubernetes/combined.yaml
	  	else
			$(K8S_CLI) apply -f https://raw.githubusercontent.com/devfile/devworkspace-operator/main/deploy/deployment/kubernetes/combined.yaml
	  	fi
		$(MAKE) wait-devworkspace-running NAMESPACE="devworkspace-controller"
	else
		if [[ $(CHANNEL) == "fast" ]]; then
			IMAGE="quay.io/devfile/devworkspace-operator-index:release"
		else
			IMAGE="quay.io/devfile/devworkspace-operator-index:next"
		fi

		$(MAKE) create-catalogsource IMAGE="$${IMAGE}" NAME="devworkspace-operator" NAMESPACE="openshift-marketplace"
		$(MAKE) create-subscription \
			NAME="devworkspace-operator" \
			NAMESPACE=$${DEFINED_OPERATOR_NAMESPACE} \
			PACKAGE_NAME="devworkspace-operator" \
			CHANNEL=$(CHANNEL) \
			SOURCE="devworkspace-operator" \
			SOURCE_NAMESPACE="openshift-marketplace" \
			INSTALL_PLAN_APPROVAL="Auto"
		$(MAKE) wait-devworkspace-running NAMESPACE=$${DEFINED_OPERATOR_NAMESPACE}
	fi

wait-devworkspace-running: SHELL := /bin/bash
wait-devworkspace-running: ## Wait until Dev Workspace operator is up and running
	$(MAKE) wait-pod-running SELECTOR="app.kubernetes.io/name=devworkspace-controller" NAMESPACE=$(NAMESPACE)
	$(MAKE) wait-pod-running SELECTOR="app.kubernetes.io/name=devworkspace-webhook-server" NAMESPACE=$(NAMESPACE)

setup-checluster: create-namespace create-checluster-crd create-checluster-cr ## Setup CheCluster (creates namespace, CRD and CheCluster CR)

create-checluster-crd: SHELL := /bin/bash
create-checluster-crd: ## Creates CheCluster Custom Resource Definition
	if [[ $(PLATFORM) == "kubernetes" ]]; then
		$(MAKE) install-certmgr
		$(K8S_CLI) apply -f $(DEPLOYMENT_DIR)/$(PLATFORM)/objects/che-operator-selfsigned-issuer.Issuer.yaml
		$(K8S_CLI) apply -f $(DEPLOYMENT_DIR)/$(PLATFORM)/objects/che-operator-serving-cert.Certificate.yaml
	fi
	$(K8S_CLI) apply -f $(DEPLOYMENT_DIR)/$(PLATFORM)/objects/checlusters.org.eclipse.che.CustomResourceDefinition.yaml

create-checluster-cr: SHELL := /bin/bash
create-checluster-cr: ## Creates CheCluster Custom Resource V2
	if [[ "$$($(K8S_CLI) get checluster eclipse-che -n $(ECLIPSE_CHE_NAMESPACE) || false )" ]]; then
		echo "[INFO] CheCluster already exists."
	else
		CHECLUSTER_CR_2_APPLY=/tmp/checluster_cr.yaml
		cp  $(CHECLUSTER_CR_PATH) $${CHECLUSTER_CR_2_APPLY}

		# Update networking.domain field with an actual value
		if [[ $(PLATFORM) == "kubernetes" ]]; then
  			# kubectl does not have `whoami` command
			CLUSTER_API_URL=$$(oc whoami --show-server=true) || true;
			CLUSTER_DOMAIN=$$(echo $${CLUSTER_API_URL} | sed -E 's/https:\/\/(.*):.*/\1/g')
			yq -riY  '.spec.networking.domain = "'$${CLUSTER_DOMAIN}'.nip.io"' $${CHECLUSTER_CR_2_APPLY}
		fi
		$(K8S_CLI) apply -f $${CHECLUSTER_CR_2_APPLY} -n $(ECLIPSE_CHE_NAMESPACE)
	fi

wait-eclipseche-version: SHELL := /bin/bash
wait-eclipseche-version: ## Wait until Eclipse Che given version is up and running
	echo "[INFO] Wait for Eclipse Che '$(VERSION)' version"

	[[ -z "$(VERSION)" ]] && { echo [ERROR] VERSION not defined; exit 1; }
	[[ -z "$(NAMESPACE)" ]] && { echo [ERROR] NAMESPACE not defined; exit 1; }

	n=0
	while [[ $$n -le 500 ]]
	do
		$(K8S_CLI) get pods -n  $(NAMESPACE)
		ACTUAL_VERSION=$$($(K8S_CLI) get checluster eclipse-che -n $(NAMESPACE) -o "jsonpath={.status.cheVersion}")
		if [[ $${ACTUAL_VERSION} == $(VERSION) ]]; then
			echo "[INFO] Eclipse Che $(VERSION) has been successfully deployed"
			break
		fi

		sleep 5
		n=$$(( n+1 ))
	done

	if [[ $$n -gt 500 ]]; then
		echo "[ERROR] Failed to deploy Eclipse Che '${version}' version"
		exit 1
	fi