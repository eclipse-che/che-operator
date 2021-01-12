//
// Copyright (c) 2020-2020 Red Hat, Inc.
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
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/gateway"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

func Expose(
	deployContext *deploy.DeployContext,
	cheHost string,
	endpointName string,
	additionalLabels string,
	component string) (endpont string, done bool, err error) {
	exposureStrategy := util.GetServerExposureStrategy(deployContext.CheCluster, deploy.DefaultServerExposureStrategy)
	var domain string
	var endpoint string
	var pathPrefix string
	var stripPrefix bool

	if endpointName == deploy.IdentityProviderName {
		pathPrefix = "auth"
		stripPrefix = false
	} else {
		pathPrefix = endpointName
		stripPrefix = true
	}
	if exposureStrategy == "multi-host" {
		// this won't get used on openshift, because there we're intentionally let Openshift decide on the domain name
		domain = endpointName + "-" + deployContext.CheCluster.Namespace + "." + deployContext.CheCluster.Spec.K8s.IngressDomain
		endpoint = domain
	} else {
		domain = cheHost
		if endpointName == deploy.IdentityProviderName {
			// legacy
			endpoint = domain
		} else {
			endpoint = domain + "/" + pathPrefix
		}
	}

	gatewayConfig := "che-gateway-route-" + endpointName
	singleHostExposureType := deploy.GetSingleHostExposureType(deployContext.CheCluster)
	useGateway := exposureStrategy == "single-host" && (util.IsOpenShift || singleHostExposureType == "gateway")

	if !util.IsOpenShift {
		if useGateway {
			cfg := gateway.GetGatewayRouteConfig(deployContext, gatewayConfig, "/"+pathPrefix, 10, "http://"+endpointName+":8080", stripPrefix)
			clusterCfg, err := deploy.SyncConfigMapToCluster(deployContext, &cfg)
			if !util.IsTestMode() {
				if clusterCfg == nil {
					if err != nil {
						logrus.Error(err)
					}
					return "", false, err
				}
			}
			if err := deploy.DeleteIngressIfExists(endpointName, deployContext); !util.IsTestMode() && err != nil {
				logrus.Error(err)
			}
		} else {
			ingress, err := deploy.SyncIngressToCluster(deployContext, endpointName, domain, endpointName, 8080, additionalLabels, component)
			if !util.IsTestMode() {
				if ingress == nil {
					logrus.Infof("Waiting on ingress '%s' to be ready", endpointName)
					if err != nil {
						logrus.Error(err)
					}
					return "", false, err
				}
			}
			if err := gateway.DeleteGatewayRouteConfig(gatewayConfig, deployContext); !util.IsTestMode() && err != nil {
				logrus.Error(err)
			}
		}
	} else {
		if useGateway {
			cfg := gateway.GetGatewayRouteConfig(deployContext, gatewayConfig, "/"+pathPrefix, 10, "http://"+endpointName+":8080", stripPrefix)
			clusterCfg, err := deploy.SyncConfigMapToCluster(deployContext, &cfg)
			if !util.IsTestMode() {
				if clusterCfg == nil {
					if err != nil {
						logrus.Error(err)
					}
					return "", false, err
				}
			}
			if err := deploy.DeleteRouteIfExists(endpointName, deployContext); !util.IsTestMode() && err != nil {
				logrus.Error(err)
			}
		} else {
			// the empty string for a host is intentional here - we let OpenShift decide on the hostname
			route, err := deploy.SyncRouteToCluster(deployContext, endpointName, "", endpointName, 8080, additionalLabels, component)
			if route == nil {
				logrus.Infof("Waiting on route '%s' to be ready", endpointName)
				if err != nil {
					logrus.Error(err)
				}
				return "", false, err
			}

			if err := gateway.DeleteGatewayRouteConfig(gatewayConfig, deployContext); !util.IsTestMode() && err != nil {
				logrus.Error(err)
			}

			endpoint = route.Spec.Host
		}
	}
	return endpoint, true, nil
}
