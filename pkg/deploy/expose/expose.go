//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	routev1 "github.com/openshift/api/route/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	networking "k8s.io/api/networking/v1"
)

const (
	gatewayConfigComponentName = "che-gateway-config"
)

//Expose exposes the specified component according to the configured exposure strategy rules
func Expose(
	deployContext *deploy.DeployContext,
	componentName string,
	routeCustomSettings orgv1.RouteCustomSettings,
	ingressCustomSettings orgv1.IngressCustomSettings,
	gatewayConfig *gateway.TraefikConfig) (endpointUrl string, done bool, err error) {
	//the host and path are empty and will be evaluated for the specified component + path
	return ExposeWithHostPath(deployContext, componentName, "", "", routeCustomSettings, ingressCustomSettings, gatewayConfig)
}

//Expose exposes the specified component on the specified host and domain.
//Empty host or path will be evaluated according to the configured strategy rules.
//Note: path may be prefixed according to the configured strategy rules.
func ExposeWithHostPath(
	deployContext *deploy.DeployContext,
	component string,
	host string,
	path string,
	routeCustomSettings orgv1.RouteCustomSettings,
	ingressCustomSettings orgv1.IngressCustomSettings,
	gatewayConfig *gateway.TraefikConfig) (endpointUrl string, done bool, err error) {

	exposureStrategy := util.GetServerExposureStrategy(deployContext.CheCluster)

	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	singleHostExposureType := deploy.GetSingleHostExposureType(deployContext.CheCluster)
	useGateway := exposureStrategy == "single-host" && (util.IsOpenShift || singleHostExposureType == deploy.GatewaySingleHostExposureType)
	if !util.IsOpenShift {
		if useGateway {
			return exposeWithGateway(deployContext, gatewayConfig, component, path, func() {
				if _, err = deploy.DeleteNamespacedObject(deployContext, component, &networking.Ingress{}); err != nil {
					logrus.Error(err)
				}
			})
		} else {
			endpointUrl, done, err = deploy.SyncIngressToCluster(deployContext, component, host, path, component, 8080, ingressCustomSettings, component)
			if !done {
				logrus.Infof("Waiting on ingress '%s' to be ready", component)
				if err != nil {
					logrus.Error(err)
				}
				return "", false, err
			}
			if err := gateway.DeleteGatewayRouteConfig(component, deployContext); !util.IsTestMode() && err != nil {
				logrus.Error(err)
			}

			return endpointUrl, true, nil
		}
	} else {
		if useGateway {
			return exposeWithGateway(deployContext, gatewayConfig, component, path, func() {
				if _, err := deploy.DeleteNamespacedObject(deployContext, component, &routev1.Route{}); !util.IsTestMode() && err != nil {
					logrus.Error(err)
				}
			})
		} else {
			// the empty string for a host is intentional here - we let OpenShift decide on the hostname
			done, err := deploy.SyncRouteToCluster(deployContext, component, host, path, component, 8080, routeCustomSettings, component)
			if !done {
				logrus.Infof("Waiting on route '%s' to be ready", component)
				if err != nil {
					logrus.Error(err)
				}
				return "", false, err
			}

			route := &routev1.Route{}
			exists, err := deploy.GetNamespacedObject(deployContext, component, route)
			if !exists {
				if err != nil {
					logrus.Error(err)
				}
				return "", false, err
			}

			if err := gateway.DeleteGatewayRouteConfig(component, deployContext); !util.IsTestMode() && err != nil {
				logrus.Error(err)
			}

			// Keycloak needs special rule in multihost. It's exposed on / which redirects to /auth
			// clients which does not support redirects needs /auth be explicitely set
			if path == "" && component == deploy.IdentityProviderName {
				path = "/auth"
			}
			return route.Spec.Host + path, true, nil
		}
	}
}

func exposeWithGateway(deployContext *deploy.DeployContext,
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
		if component == deploy.IdentityProviderName {
			path = "/auth"
		} else {
			path = "/" + component
		}
	}
	return deployContext.CheCluster.Spec.Server.CheHost + path, true, err
}
