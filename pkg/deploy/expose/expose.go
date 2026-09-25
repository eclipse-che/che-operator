//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package expose

import (
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/diffs"

	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	routev1 "github.com/openshift/api/route/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	logger = ctrl.Log.WithName("expose")
)

// Expose exposes the specified component according to the configured exposure strategy rules
func Expose(
	deployContext *chetypes.DeployContext,
	componentName string,
	gatewayConfig *gateway.TraefikConfig) (endpointUrl string, done bool, err error) {
	//the host and path are empty and will be evaluated for the specified component + path
	return ExposeWithHostPath(deployContext, componentName, "", "", gatewayConfig)
}

// Expose exposes the specified component on the specified host and domain.
// Empty host or path will be evaluated according to the configured strategy rules.
// Note: path may be prefixed according to the configured strategy rules.
func ExposeWithHostPath(
	deployContext *chetypes.DeployContext,
	component string,
	host string,
	path string,
	gatewayConfig *gateway.TraefikConfig) (endpointUrl string, done bool, err error) {

	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	key := types.NamespacedName{Name: component, Namespace: deployContext.CheCluster.Namespace}
	clientWrapper := deployContext.ClusterAPI.ClientWrapper

	if !infrastructure.IsOpenShift() {
		return exposeWithGateway(deployContext, gatewayConfig, component, path, func() {
			if err := clientWrapper.DeleteByKeyIgnoreNotFound(deployContext.Context, key, &networking.Ingress{}); err != nil {
				logger.Error(err, "Failed to delete Ingress", "namespace", key.Namespace, "name", key.Name)
			}
		})
	} else {
		return exposeWithGateway(deployContext, gatewayConfig, component, path, func() {
			if err := clientWrapper.DeleteByKeyIgnoreNotFound(deployContext.Context, key, &routev1.Route{}); err != nil {
				logger.Error(err, "Failed to delete Route", "namespace", key.Namespace, "name", key.Name)
			}
		})
	}
}

func exposeWithGateway(deployContext *chetypes.DeployContext,
	gatewayConfig *gateway.TraefikConfig,
	component string,
	path string,
	cleanUpRouting func()) (endpointUrl string, done bool, err error) {

	cfg, err := gateway.GetConfigmapForGatewayConfig(deployContext, component, gatewayConfig)
	if err != nil {
		return "", false, fmt.Errorf("failed to get gateway ConfigMap for component %s: %w", component, err)
	}

	if err := controllerutil.SetControllerReference(deployContext.CheCluster, cfg, deployContext.ClusterAPI.Scheme); err != nil {
		return "", false, fmt.Errorf("failed to set owner reference for ConfigMap %s/%s: %w", cfg.Namespace, cfg.Name, err)
	}

	if err := deployContext.ClusterAPI.ClientWrapper.Sync(
		deployContext.Context,
		cfg,
		&k8sclient.SyncOptions{DiffOpts: diffs.ConfigMapEnsureLabels},
	); err != nil {
		return "", false, fmt.Errorf("failed to sync ConfigMap %s/%s: %w", cfg.Namespace, cfg.Name, err)
	}

	cleanUpRouting()

	if path == "" {
		path = "/" + component
	}
	return deployContext.CheHost + path, true, nil
}
