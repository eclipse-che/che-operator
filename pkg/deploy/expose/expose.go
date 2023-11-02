//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	routev1 "github.com/openshift/api/route/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/sirupsen/logrus"
	networking "k8s.io/api/networking/v1"
)

const (
	gatewayConfigComponentName = "che-gateway-config"
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

	if !infrastructure.IsOpenShift() {
		return exposeWithGateway(deployContext, gatewayConfig, component, path, func() {
			if _, err = deploy.DeleteNamespacedObject(deployContext, component, &networking.Ingress{}); err != nil {
				logrus.Error(err)
			}
		})
	} else {
		return exposeWithGateway(deployContext, gatewayConfig, component, path, func() {
			if _, err := deploy.DeleteNamespacedObject(deployContext, component, &routev1.Route{}); !test.IsTestMode() && err != nil {
				logrus.Error(err)
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
		return "", false, err
	}
	done, err = deploy.SyncConfigMapSpecToCluster(deployContext, &cfg)
	if !done {
		if err != nil {
			logrus.Error(err)
		}
		return "", false, err
	}

	cleanUpRouting()

	if path == "" {
		path = "/" + component
	}
	return deployContext.CheHost + path, true, err
}
