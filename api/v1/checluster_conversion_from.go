//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package v1

import (
	"context"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/utils"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func (dst *CheCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*chev2.CheCluster)
	dst.ObjectMeta = src.ObjectMeta

	if err := dst.convertFrom_Server(src); err != nil {
		return err
	}

	if err := dst.convertFrom_K8s(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Auth(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Database(src); err != nil {
		return err
	}

	if err := dst.convertFrom_DevWorkspace(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Dashboard(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Metrics(src); err != nil {
		return err
	}

	if err := dst.convertFrom_ImagePuller(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Storage(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Status(src); err != nil {
		return err
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server(src *chev2.CheCluster) error {
	dst.Spec.Server.AirGapContainerRegistryHostname = src.Spec.ContainerRegistry.Hostname
	dst.Spec.Server.AirGapContainerRegistryOrganization = src.Spec.ContainerRegistry.Organization
	dst.Spec.Server.CheClusterRoles = strings.Join(src.Spec.Operands.CheServer.ClusterRoles, ",")
	dst.Spec.Server.CustomCheProperties = utils.CloneMap(src.Spec.Operands.CheServer.ExtraProperties)
	dst.Spec.Server.CheDebug = strconv.FormatBool(src.Spec.Operands.CheServer.Debug)
	dst.Spec.Server.CheLogLevel = src.Spec.Operands.CheServer.LogLevel
	dst.Spec.Server.ProxyURL = src.Spec.Operands.CheServer.Proxy.Url
	dst.Spec.Server.ProxyPort = src.Spec.Operands.CheServer.Proxy.Port
	dst.Spec.Server.NonProxyHosts = strings.Join(src.Spec.Operands.CheServer.Proxy.NonProxyHosts, "|")
	dst.Spec.Server.ProxySecret = src.Spec.Operands.CheServer.Proxy.CredentialsSecretName
	dst.Spec.Server.WorkspaceNamespaceDefault = src.Spec.Workspaces.DefaultNamespace.Template
	dst.Spec.Server.WorkspacePodNodeSelector = utils.CloneMap(src.Spec.Workspaces.NodeSelector)

	dst.Spec.Server.WorkspacePodTolerations = []corev1.Toleration{}
	for _, v := range src.Spec.Workspaces.Tolerations {
		dst.Spec.Server.WorkspacePodTolerations = append(dst.Spec.Server.WorkspacePodTolerations, v)
	}

	dst.Spec.Server.WorkspacesDefaultPlugins = make([]WorkspacesDefaultPlugins, 0)
	for _, p := range src.Spec.Workspaces.DefaultPlugins {
		dst.Spec.Server.WorkspacesDefaultPlugins = append(dst.Spec.Server.WorkspacesDefaultPlugins,
			WorkspacesDefaultPlugins{
				Editor:  p.Editor,
				Plugins: p.Plugins,
			})
	}

	if len(src.Spec.Operands.CheServer.Deployment.Containers) != 0 {
		cheServerImageAndTag := strings.Split(src.Spec.Operands.CheServer.Deployment.Containers[0].Image, ":")
		dst.Spec.Server.CheImage = strings.Join(cheServerImageAndTag[0:len(cheServerImageAndTag)-1], ":")
		dst.Spec.Server.CheImageTag = cheServerImageAndTag[len(cheServerImageAndTag)-1]
		dst.Spec.Server.CheImagePullPolicy = src.Spec.Operands.CheServer.Deployment.Containers[0].ImagePullPolicy
		dst.Spec.Server.ServerMemoryRequest = src.Spec.Operands.CheServer.Deployment.Containers[0].Resources.Requests.Memory
		dst.Spec.Server.ServerCpuRequest = src.Spec.Operands.CheServer.Deployment.Containers[0].Resources.Requests.Cpu
		dst.Spec.Server.ServerMemoryLimit = src.Spec.Operands.CheServer.Deployment.Containers[0].Resources.Limits.Memory
		dst.Spec.Server.ServerCpuLimit = src.Spec.Operands.CheServer.Deployment.Containers[0].Resources.Limits.Cpu
	}

	if infrastructure.IsOpenShift() {
		dst.Spec.Server.CheHost = src.Spec.Ingress.Hostname
		dst.Spec.Server.CheServerRoute.Labels = labels.FormatLabels(src.Spec.Ingress.Labels)
		dst.Spec.Server.CheServerRoute.Annotations = utils.CloneMap(src.Spec.Ingress.Annotations)
		dst.Spec.Server.CheServerRoute.Domain = src.Spec.Ingress.Domain
		dst.Spec.Server.CheHostTLSSecret = src.Spec.Ingress.TlsSecretName
	} else {
		dst.Spec.Server.CheHost = src.Spec.Ingress.Hostname
		dst.Spec.Server.CheServerIngress.Labels = labels.FormatLabels(src.Spec.Ingress.Labels)
		dst.Spec.Server.CheServerIngress.Annotations = utils.CloneMap(src.Spec.Ingress.Annotations)
	}

	for _, c := range src.Spec.Ingress.Auth.Gateway.Deployment.Containers {
		switch c.Name {
		case constants.GatewayContainerName:
			dst.Spec.Server.SingleHostGatewayImage = c.Image
		case constants.GatewayConfigSideCarContainerName:
			dst.Spec.Server.SingleHostGatewayConfigSidecarImage = c.Image
		}
	}

	dst.Spec.Server.SingleHostGatewayConfigMapLabels = utils.CloneMap(src.Spec.Ingress.Auth.Gateway.ConfigLabels)

	trustStoreConfigMap, err := findTrustStoreConfigMap(src.Namespace)
	if err != nil {
		return err
	} else {
		dst.Spec.Server.ServerTrustStoreConfigMapName = trustStoreConfigMap
	}

	if src.Spec.Workspaces.TrustedCerts.GitTrustedCertsConfigMapName != "" {
		dst.Spec.Server.GitSelfSignedCert = true
	}

	if err := dst.convertFrom_Server_PluginRegistry(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Server_DevfileRegistry(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Server_Dashboard(src); err != nil {
		return err
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server_PluginRegistry(src *chev2.CheCluster) error {
	dst.Spec.Server.ExternalPluginRegistry = src.Spec.Operands.PluginRegistry.DisableInternalRegistry

	if src.Spec.Operands.PluginRegistry.DisableInternalRegistry {
		if len(src.Spec.Operands.PluginRegistry.ExternalPluginRegistries) != 0 {
			dst.Spec.Server.PluginRegistryUrl = src.Spec.Operands.PluginRegistry.ExternalPluginRegistries[0].Url
		}
	}

	if len(src.Spec.Operands.PluginRegistry.Deployment.Containers) != 0 {
		dst.Spec.Server.PluginRegistryImage = src.Spec.Operands.PluginRegistry.Deployment.Containers[0].Image
		dst.Spec.Server.PluginRegistryPullPolicy = src.Spec.Operands.PluginRegistry.Deployment.Containers[0].ImagePullPolicy
		dst.Spec.Server.PluginRegistryMemoryRequest = src.Spec.Operands.PluginRegistry.Deployment.Containers[0].Resources.Requests.Memory
		dst.Spec.Server.PluginRegistryCpuRequest = src.Spec.Operands.PluginRegistry.Deployment.Containers[0].Resources.Requests.Cpu
		dst.Spec.Server.PluginRegistryMemoryLimit = src.Spec.Operands.PluginRegistry.Deployment.Containers[0].Resources.Limits.Memory
		dst.Spec.Server.PluginRegistryCpuLimit = src.Spec.Operands.PluginRegistry.Deployment.Containers[0].Resources.Limits.Cpu
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server_DevfileRegistry(src *chev2.CheCluster) error {
	dst.Spec.Server.ExternalDevfileRegistry = src.Spec.Operands.DevfileRegistry.DisableInternalRegistry

	dst.Spec.Server.ExternalDevfileRegistries = make([]ExternalDevfileRegistries, 0)
	for _, r := range src.Spec.Operands.DevfileRegistry.ExternalDevfileRegistries {
		dst.Spec.Server.ExternalDevfileRegistries = append(dst.Spec.Server.ExternalDevfileRegistries,
			ExternalDevfileRegistries{
				Url: r.Url,
			})
	}

	if len(src.Spec.Operands.DevfileRegistry.Deployment.Containers) != 0 {
		dst.Spec.Server.DevfileRegistryImage = src.Spec.Operands.DevfileRegistry.Deployment.Containers[0].Image
		dst.Spec.Server.DevfileRegistryPullPolicy = src.Spec.Operands.DevfileRegistry.Deployment.Containers[0].ImagePullPolicy
		dst.Spec.Server.DevfileRegistryMemoryRequest = src.Spec.Operands.DevfileRegistry.Deployment.Containers[0].Resources.Requests.Memory
		dst.Spec.Server.DevfileRegistryCpuRequest = src.Spec.Operands.DevfileRegistry.Deployment.Containers[0].Resources.Requests.Cpu
		dst.Spec.Server.DevfileRegistryMemoryLimit = src.Spec.Operands.DevfileRegistry.Deployment.Containers[0].Resources.Limits.Memory
		dst.Spec.Server.DevfileRegistryCpuLimit = src.Spec.Operands.DevfileRegistry.Deployment.Containers[0].Resources.Limits.Cpu
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server_Dashboard(src *chev2.CheCluster) error {
	if len(src.Spec.Operands.Dashboard.Deployment.Containers) != 0 {
		dst.Spec.Server.DashboardImage = src.Spec.Operands.Dashboard.Deployment.Containers[0].Image
		dst.Spec.Server.DashboardImagePullPolicy = string(src.Spec.Operands.Dashboard.Deployment.Containers[0].ImagePullPolicy)
		dst.Spec.Server.DashboardMemoryRequest = src.Spec.Operands.Dashboard.Deployment.Containers[0].Resources.Requests.Memory
		dst.Spec.Server.DashboardCpuRequest = src.Spec.Operands.Dashboard.Deployment.Containers[0].Resources.Requests.Cpu
		dst.Spec.Server.DashboardMemoryLimit = src.Spec.Operands.Dashboard.Deployment.Containers[0].Resources.Limits.Memory
		dst.Spec.Server.DashboardCpuLimit = src.Spec.Operands.Dashboard.Deployment.Containers[0].Resources.Limits.Cpu
	}

	return nil
}

func (dst *CheCluster) convertFrom_K8s(src *chev2.CheCluster) error {
	if src.Spec.Operands.CheServer.Deployment.SecurityContext.RunAsUser != nil {
		dst.Spec.K8s.SecurityContextRunAsUser = strconv.FormatInt(*src.Spec.Operands.CheServer.Deployment.SecurityContext.RunAsUser, 10)
	}

	if src.Spec.Operands.CheServer.Deployment.SecurityContext.FsGroup != nil {
		dst.Spec.K8s.SecurityContextFsGroup = strconv.FormatInt(*src.Spec.Operands.CheServer.Deployment.SecurityContext.FsGroup, 10)
	}

	if !infrastructure.IsOpenShift() {
		dst.Spec.K8s.IngressDomain = src.Spec.Ingress.Domain
		dst.Spec.K8s.TlsSecretName = src.Spec.Ingress.TlsSecretName
		delete(dst.Spec.Server.CheServerIngress.Annotations, "kubernetes.io/ingress.class")
		dst.Spec.K8s.IngressClass = src.Spec.Ingress.Annotations["kubernetes.io/ingress.class"]
		if dst.Spec.K8s.IngressClass == "" {
			dst.Spec.K8s.IngressClass = constants.DefaultIngressClass
		}
	}

	return nil
}

func (dst *CheCluster) convertFrom_Auth(src *chev2.CheCluster) error {
	dst.Spec.Auth.IdentityProviderURL = src.Spec.Ingress.Auth.IdentityProviderURL
	dst.Spec.Auth.OAuthClientName = src.Spec.Ingress.Auth.OAuthClientName
	dst.Spec.Auth.OAuthSecret = src.Spec.Ingress.Auth.OAuthSecret

	for _, c := range src.Spec.Ingress.Auth.Gateway.Deployment.Containers {
		switch c.Name {
		case constants.GatewayAuthenticationContainerName:
			dst.Spec.Auth.GatewayAuthenticationSidecarImage = c.Image
		case constants.GatewayAuthorizationContainerName:
			dst.Spec.Auth.GatewayAuthorizationSidecarImage = c.Image
		}
	}

	return nil
}

func (dst *CheCluster) convertFrom_Database(src *chev2.CheCluster) error {
	dst.Spec.Database.ExternalDb = src.Spec.Operands.Database.ExternalDb
	dst.Spec.Database.ChePostgresDb = src.Spec.Operands.Database.PostgresDb
	dst.Spec.Database.ChePostgresHostName = src.Spec.Operands.Database.PostgresHostName
	dst.Spec.Database.ChePostgresPort = src.Spec.Operands.Database.PostgresPort
	dst.Spec.Database.PostgresVersion = src.Status.PostgresVersion
	dst.Spec.Database.PvcClaimSize = src.Spec.Operands.Database.Pvc.ClaimSize
	dst.Spec.Database.ChePostgresSecret = src.Spec.Operands.Database.CredentialsSecretName

	if len(src.Spec.Operands.Database.Deployment.Containers) != 0 {
		dst.Spec.Database.PostgresImage = src.Spec.Operands.Database.Deployment.Containers[0].Image
		dst.Spec.Database.PostgresImagePullPolicy = src.Spec.Operands.Database.Deployment.Containers[0].ImagePullPolicy
		dst.Spec.Database.ChePostgresContainerResources.Requests.Memory = src.Spec.Operands.Database.Deployment.Containers[0].Resources.Requests.Memory
		dst.Spec.Database.ChePostgresContainerResources.Requests.Cpu = src.Spec.Operands.Database.Deployment.Containers[0].Resources.Requests.Cpu
		dst.Spec.Database.ChePostgresContainerResources.Limits.Memory = src.Spec.Operands.Database.Deployment.Containers[0].Resources.Limits.Memory
		dst.Spec.Database.ChePostgresContainerResources.Limits.Cpu = src.Spec.Operands.Database.Deployment.Containers[0].Resources.Limits.Cpu
	}

	return nil
}

func (dst *CheCluster) convertFrom_DevWorkspace(src *chev2.CheCluster) error {
	if len(src.Spec.Operands.DevWorkspace.Deployment.Containers) != 0 {
		dst.Spec.DevWorkspace.ControllerImage = src.Spec.Operands.DevWorkspace.Deployment.Containers[0].Image
	}
	dst.Spec.DevWorkspace.RunningLimit = src.Spec.Operands.DevWorkspace.RunningLimit
	dst.Spec.DevWorkspace.Enable = true

	return nil
}

func (dst *CheCluster) convertFrom_ImagePuller(src *chev2.CheCluster) error {
	dst.Spec.ImagePuller.Enable = src.Spec.Operands.ImagePuller.Enable
	dst.Spec.ImagePuller.Spec = src.Spec.Operands.ImagePuller.Spec

	return nil
}

func (dst *CheCluster) convertFrom_Metrics(src *chev2.CheCluster) error {
	dst.Spec.Metrics.Enable = src.Spec.Operands.Metrics.Enable

	return nil
}

func (dst *CheCluster) convertFrom_Dashboard(src *chev2.CheCluster) error {
	dst.Spec.Dashboard.Warning = src.Spec.Operands.Dashboard.HeaderMessage.Text

	return nil
}

func (dst *CheCluster) convertFrom_Status(src *chev2.CheCluster) error {
	dst.Status.CheURL = src.Status.CheURL
	dst.Status.CheVersion = src.Status.CheVersion
	dst.Status.DevfileRegistryURL = src.Status.DevfileRegistryURL
	dst.Status.PluginRegistryURL = src.Status.PluginRegistryURL
	dst.Status.Message = src.Status.Message
	dst.Status.Reason = src.Status.Reason
	dst.Status.DevworkspaceStatus.GatewayPhase = GatewayPhase(src.Status.GatewayPhase)
	dst.Status.DevworkspaceStatus.GatewayHost = src.GetCheHost()
	dst.Status.DevworkspaceStatus.WorkspaceBaseDomain = src.Status.WorkspaceBaseDomain
	dst.Status.DevworkspaceStatus.Message = src.Status.Message
	dst.Status.DevworkspaceStatus.Phase = ClusterPhase(src.Status.ChePhase)
	dst.Status.DevworkspaceStatus.Reason = src.Status.Reason

	switch src.Status.ChePhase {
	case chev2.ClusterPhaseActive:
		dst.Status.CheClusterRunning = "Available"
	case chev2.ClusterPhaseInactive:
		dst.Status.CheClusterRunning = "Unavailable"
	case chev2.RollingUpdate:
		dst.Status.CheClusterRunning = "Available, Rolling Update in Progress"
	}

	if src.Spec.Workspaces.TrustedCerts.GitTrustedCertsConfigMapName != "" {
		dst.Status.GitServerTLSCertificateConfigMapName = src.Spec.Workspaces.TrustedCerts.GitTrustedCertsConfigMapName
	}

	return nil
}

func (dst *CheCluster) convertFrom_Storage(src *chev2.CheCluster) error {
	dst.Spec.Storage.PostgresPVCStorageClassName = src.Spec.Operands.Database.Pvc.StorageClass
	dst.Spec.Storage.PreCreateSubPaths = src.Spec.Workspaces.Storage.PreCreateSubPaths
	dst.Spec.Storage.PvcClaimSize = src.Spec.Workspaces.Storage.Pvc.ClaimSize
	dst.Spec.Storage.WorkspacePVCStorageClassName = src.Spec.Workspaces.Storage.Pvc.StorageClass
	dst.Spec.Storage.PvcJobsImage = src.Spec.Workspaces.Storage.PvcJobsImage
	dst.Spec.Storage.PvcStrategy = src.Spec.Workspaces.Storage.PvcStrategy

	return nil
}

// Finds TrustStore ConfigMap.
func findTrustStoreConfigMap(namespace string) (string, error) {
	k8sHelper := k8shelper.New()

	_, err := k8sHelper.GetClientset().CoreV1().ConfigMaps(namespace).Get(context.TODO(), constants.DefaultServerTrustStoreConfigMapName, metav1.GetOptions{})
	if err == nil {
		// TrustStore ConfigMap with a default name exists
		return constants.DefaultServerTrustStoreConfigMapName, nil
	}

	return "", nil
}
