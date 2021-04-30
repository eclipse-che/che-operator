//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package server

import (
	"context"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

const (
	AvailableStatus               = "Available"
	UnavailableStatus             = "Unavailable"
	RollingUpdateInProgressStatus = "Available: Rolling update in progress"
)

type Server struct {
	deployContext *deploy.DeployContext
	component     string
}

func NewServer(deployContext *deploy.DeployContext) *Server {
	return &Server{
		deployContext: deployContext,
		component:     deploy.DefaultCheFlavor(deployContext.CheCluster),
	}
}

func (s *Server) SyncAll() (bool, error) {
	done, err := s.syncService()
	if !done {
		return false, err
	}

	done, err = s.syncLegacyConfigMap()
	if !done {
		return false, err
	}

	done, err = s.syncConfigMap()
	if !done {
		return false, err
	}

	done, err = s.findDefaultCheHost()
	if !done {
		return false, err
	}

	done, err = s.exposeEndpoint()
	if !done {
		return false, err
	}

	done, err = s.updateStatus()
	if !done {
		return false, err
	}

	done, err = s.syncPVC()
	if !done {
		return false, err
	}

	done, err = s.syncDeployment()
	if !done {
		return false, err
	}

	done, err = s.updateAvailabilityStatus()
	if !done {
		return false, err
	}

	done, err = s.updateCheVersion()
	if !done {
		return false, err
	}

	return true, nil
}

func (s *Server) syncService() (bool, error) {
	portName := []string{"http"}
	portNumber := []int32{8080}

	if s.deployContext.CheCluster.Spec.Metrics.Enable {
		portName = append(portName, "metrics")
		portNumber = append(portNumber, deploy.DefaultCheMetricsPort)
	}

	if s.deployContext.CheCluster.Spec.Server.CheDebug == "true" {
		portName = append(portName, "debug")
		portNumber = append(portNumber, deploy.DefaultCheDebugPort)
	}

	spec := deploy.GetServiceSpec(s.deployContext, deploy.CheServiceName, portName, portNumber, s.component)
	return deploy.Sync(s.deployContext, spec, deploy.ServiceDefaultDiffOpts)
}

func (s Server) exposeEndpoint() (bool, error) {
	cheHost := ""
	exposedServiceName := GetServerExposingServiceName(s.deployContext.CheCluster)

	if !util.IsOpenShift {
		_, done, err := deploy.SyncIngressToCluster(
			s.deployContext,
			s.component,
			s.deployContext.CheCluster.Spec.Server.CheHost,
			"/",
			exposedServiceName,
			8080,
			s.deployContext.CheCluster.Spec.Server.CheServerIngress,
			s.component)
		if !done {
			return false, err
		}

		ingress := &v1beta1.Ingress{}
		exists, err := deploy.GetNamespacedObject(s.deployContext, s.component, ingress)
		if !exists {
			return false, err
		}

		cheHost = ingress.Spec.Rules[0].Host
	} else {
		customHost := s.deployContext.CheCluster.Spec.Server.CheHost
		if s.deployContext.DefaultCheHost == customHost {
			// let OpenShift set a hostname by itself since it requires a routes/custom-host permissions
			customHost = ""
		}

		done, err := deploy.SyncRouteToCluster(
			s.deployContext,
			s.component,
			customHost,
			"/",
			exposedServiceName,
			8080,
			s.deployContext.CheCluster.Spec.Server.CheServerRoute,
			s.component)
		if !done {
			return false, err
		}

		route := &routev1.Route{}
		exists, err := deploy.GetNamespacedObject(s.deployContext, s.component, route)
		if !exists {
			return false, err
		}

		if customHost == "" {
			s.deployContext.DefaultCheHost = route.Spec.Host
		}
		cheHost = route.Spec.Host
	}

	if s.deployContext.CheCluster.Spec.Server.CheHost != cheHost {
		s.deployContext.CheCluster.Spec.Server.CheHost = cheHost
		err := deploy.UpdateCheCRSpec(s.deployContext, "CheHost URL", cheHost)
		return err == nil, err
	}

	return true, nil
}

func (s Server) updateStatus() (bool, error) {
	var cheUrl string
	if s.deployContext.CheCluster.Spec.Server.TlsSupport {
		cheUrl = "https://" + s.deployContext.CheCluster.Spec.Server.CheHost
	} else {
		cheUrl = "http://" + s.deployContext.CheCluster.Spec.Server.CheHost
	}

	if s.deployContext.CheCluster.Status.CheURL != cheUrl {
		err := deploy.UpdateCheCRStatus(s.deployContext, s.component+" server URL", cheUrl)
		return err == nil, err
	}

	return true, nil
}

func (s *Server) syncConfigMap() (bool, error) {
	data, err := s.getConfigMapData()
	if err != nil {
		return false, err
	}

	return deploy.SyncConfigMapDataToCluster(s.deployContext, CheConfigMapName, data, s.component)
}

func (s Server) syncLegacyConfigMap() (bool, error) {
	// Get custom ConfigMap
	// if it exists, add the data into CustomCheProperties
	customConfigMap := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(s.deployContext, "custom", customConfigMap)
	if err != nil {
		return false, err
	} else if exists {
		logrus.Info("Found legacy custom ConfigMap. Adding those values to CheCluster.Spec.Server.CustomCheProperties")

		if s.deployContext.CheCluster.Spec.Server.CustomCheProperties == nil {
			s.deployContext.CheCluster.Spec.Server.CustomCheProperties = make(map[string]string)
		}
		for k, v := range customConfigMap.Data {
			s.deployContext.CheCluster.Spec.Server.CustomCheProperties[k] = v
		}

		err := s.deployContext.ClusterAPI.Client.Update(context.TODO(), s.deployContext.CheCluster)
		if err != nil {
			return false, err
		}

		return deploy.DeleteNamespacedObject(s.deployContext, "custom", &corev1.ConfigMap{})
	}

	return true, nil
}

func (s Server) syncPVC() (bool, error) {
	cheMultiUser := deploy.GetCheMultiUser(s.deployContext.CheCluster)
	if cheMultiUser == "false" {
		claimSize := util.GetValue(s.deployContext.CheCluster.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
		done, err := deploy.SyncPVCToCluster(s.deployContext, deploy.DefaultCheVolumeClaimName, claimSize, s.component)
		if !done {
			if err == nil {
				logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", deploy.DefaultCheVolumeClaimName)
			}
		}
		return done, err
	} else {
		return deploy.DeleteNamespacedObject(s.deployContext, deploy.DefaultCheVolumeClaimName, &corev1.PersistentVolumeClaim{})
	}
}

func (s *Server) updateAvailabilityStatus() (bool, error) {
	cheDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(s.deployContext, s.component, cheDeployment)
	if err != nil {
		return false, err
	}

	if exists {
		if cheDeployment.Status.AvailableReplicas < 1 {
			if s.deployContext.CheCluster.Status.CheClusterRunning != UnavailableStatus {
				err := deploy.UpdateCheCRStatus(s.deployContext, "status: Che API", UnavailableStatus)
				return err == nil, err
			}
		} else if cheDeployment.Status.Replicas != 1 {
			if s.deployContext.CheCluster.Status.CheClusterRunning != RollingUpdateInProgressStatus {
				err := deploy.UpdateCheCRStatus(s.deployContext, "status: Che API", RollingUpdateInProgressStatus)
				return err == nil, err
			}
		} else {
			if s.deployContext.CheCluster.Status.CheClusterRunning != AvailableStatus {
				err := deploy.UpdateCheCRStatus(s.deployContext, "status: Che API", AvailableStatus)
				return err == nil, err
			}

			cheFlavor := deploy.DefaultCheFlavor(s.deployContext.CheCluster)
			name := "Eclipse Che"
			if cheFlavor == "codeready" {
				name = "CodeReady Workspaces"
			}

			logrus.Infof(name+" is now available at: %s", s.deployContext.CheCluster.Status.CheURL)
		}
	} else {
		err := deploy.UpdateCheCRStatus(s.deployContext, "status: Che API", UnavailableStatus)
		return err == nil, err
	}

	return true, nil
}

func (s *Server) syncDeployment() (bool, error) {
	spec, err := s.getDeploymentSpec()
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(s.deployContext, spec, deploy.DefaultDeploymentDiffOpts)
}

func (s *Server) findDefaultCheHost() (bool, error) {
	// only for OpenShift
	if !util.IsOpenShift || s.deployContext.DefaultCheHost != "" {
		return true, nil
	}

	done, err := deploy.SyncRouteToCluster(
		s.deployContext,
		s.component,
		"",
		"",
		GetServerExposingServiceName(s.deployContext.CheCluster),
		8080,
		s.deployContext.CheCluster.Spec.Server.CheServerRoute,
		s.component)
	if !done {
		return false, err
	}

	route := &routev1.Route{}
	exists, err := deploy.GetNamespacedObject(s.deployContext, s.component, route)
	if !exists {
		return false, err
	}

	s.deployContext.DefaultCheHost = route.Spec.Host
	return true, nil
}

func (s Server) updateCheVersion() (bool, error) {
	cheVersion := s.evaluateCheServerVersion()
	if s.deployContext.CheCluster.Status.CheVersion != cheVersion {
		s.deployContext.CheCluster.Status.CheVersion = cheVersion
		err := deploy.UpdateCheCRStatus(s.deployContext, "version", cheVersion)
		return err == nil, err
	}

	return true, nil
}

// EvaluateCheServerVersion evaluate che version
// based on Checluster information and image defaults from env variables
func (s Server) evaluateCheServerVersion() string {
	return util.GetValue(s.deployContext.CheCluster.Spec.Server.CheImageTag, deploy.DefaultCheVersion())
}

func GetServerExposingServiceName(cr *orgv1.CheCluster) string {
	if util.GetServerExposureStrategy(cr) == "single-host" && deploy.GetSingleHostExposureType(cr) == "gateway" {
		return gateway.GatewayServiceName
	}
	return deploy.CheServiceName
}
