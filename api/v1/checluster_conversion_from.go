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

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/eclipse-che/che-operator/pkg/common/utils"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if err := dst.convertFrom_GitServices(src); err != nil {
		return err
	}

	if err := dst.convertFrom_Status(src); err != nil {
		return err
	}

	return nil
}

func (dst *CheCluster) convertFrom_GitServices(src *chev2.CheCluster) error {
	for _, github := range src.Spec.GitServices.GitHub {
		dst.Spec.GitServices.GitHub = append(
			dst.Spec.GitServices.GitHub,
			GitHubService{
				SecretName: github.SecretName,
			})
	}

	for _, gitlab := range src.Spec.GitServices.GitLab {
		dst.Spec.GitServices.GitLab = append(
			dst.Spec.GitServices.GitLab,
			GitLabService{
				SecretName: gitlab.SecretName,
			})
	}

	for _, bitbucket := range src.Spec.GitServices.BitBucket {
		dst.Spec.GitServices.BitBucket = append(
			dst.Spec.GitServices.BitBucket,
			BitBucketService{
				SecretName: bitbucket.SecretName,
			})
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server(src *chev2.CheCluster) error {
	dst.Spec.Server.AirGapContainerRegistryHostname = src.Spec.ContainerRegistry.Hostname
	dst.Spec.Server.AirGapContainerRegistryOrganization = src.Spec.ContainerRegistry.Organization
	dst.Spec.Server.CheClusterRoles = strings.Join(src.Spec.Components.CheServer.ClusterRoles, ",")
	if src.Spec.DevEnvironments.User != nil {
		dst.Spec.Server.CheWorkspaceClusterRole = strings.Join(src.Spec.DevEnvironments.User.ClusterRoles, ",")
	}
	dst.Spec.Server.CustomCheProperties = utils.CloneMap(src.Spec.Components.CheServer.ExtraProperties)
	if src.Spec.Components.CheServer.Debug != nil {
		dst.Spec.Server.CheDebug = strconv.FormatBool(*src.Spec.Components.CheServer.Debug)
	}
	dst.Spec.Server.CheLogLevel = src.Spec.Components.CheServer.LogLevel
	if src.Spec.Components.CheServer.Proxy != nil {
		dst.Spec.Server.ProxyURL = src.Spec.Components.CheServer.Proxy.Url
		dst.Spec.Server.ProxyPort = src.Spec.Components.CheServer.Proxy.Port
		dst.Spec.Server.NonProxyHosts = strings.Join(src.Spec.Components.CheServer.Proxy.NonProxyHosts, "|")
		dst.Spec.Server.ProxySecret = src.Spec.Components.CheServer.Proxy.CredentialsSecretName
	}
	dst.Spec.Server.AllowAutoProvisionUserNamespace = src.Spec.DevEnvironments.DefaultNamespace.AutoProvision
	dst.Spec.Server.WorkspaceNamespaceDefault = src.Spec.DevEnvironments.DefaultNamespace.Template
	dst.Spec.Server.WorkspacePodNodeSelector = utils.CloneMap(src.Spec.DevEnvironments.NodeSelector)

	for _, v := range src.Spec.DevEnvironments.Tolerations {
		dst.Spec.Server.WorkspacePodTolerations = append(dst.Spec.Server.WorkspacePodTolerations, v)
	}

	for _, p := range src.Spec.DevEnvironments.DefaultPlugins {
		dst.Spec.Server.WorkspacesDefaultPlugins = append(dst.Spec.Server.WorkspacesDefaultPlugins,
			WorkspacesDefaultPlugins{
				Editor:  p.Editor,
				Plugins: p.Plugins,
			})
	}

	dst.Spec.Server.WorkspaceDefaultEditor = src.Spec.DevEnvironments.DefaultEditor
	dst.Spec.Server.WorkspaceDefaultComponents = src.Spec.DevEnvironments.DefaultComponents

	if src.Spec.Components.CheServer.Deployment != nil {
		if len(src.Spec.Components.CheServer.Deployment.Containers) != 0 {
			dst.Spec.Server.CheServerEnv = src.Spec.Components.CheServer.Deployment.Containers[0].Env
			cheServerImageAndTag := strings.Split(src.Spec.Components.CheServer.Deployment.Containers[0].Image, ":")
			dst.Spec.Server.CheImage = strings.Join(cheServerImageAndTag[0:len(cheServerImageAndTag)-1], ":")
			dst.Spec.Server.CheImageTag = cheServerImageAndTag[len(cheServerImageAndTag)-1]
			dst.Spec.Server.CheImagePullPolicy = src.Spec.Components.CheServer.Deployment.Containers[0].ImagePullPolicy

			if src.Spec.Components.CheServer.Deployment.Containers[0].Resources != nil {
				if src.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests != nil {
					dst.Spec.Server.ServerMemoryRequest = resource2String(src.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests.Memory)
					dst.Spec.Server.ServerCpuRequest = resource2String(src.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests.Cpu)
				}
				if src.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits != nil {
					dst.Spec.Server.ServerMemoryLimit = resource2String(src.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits.Memory)
					dst.Spec.Server.ServerCpuLimit = resource2String(src.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits.Cpu)
				}
			}
		}
	}

	if infrastructure.IsOpenShift() {
		dst.Spec.Server.CheHost = src.Spec.Networking.Hostname
		dst.Spec.Server.CheServerRoute.Labels = utils.FormatLabels(src.Spec.Networking.Labels)
		dst.Spec.Server.CheServerRoute.Annotations = utils.CloneMap(src.Spec.Networking.Annotations)
		dst.Spec.Server.CheServerRoute.Domain = src.Spec.Networking.Domain
		dst.Spec.Server.CheHostTLSSecret = src.Spec.Networking.TlsSecretName
	} else {
		dst.Spec.Server.CheHost = src.Spec.Networking.Hostname
		dst.Spec.Server.CheServerIngress.Labels = utils.FormatLabels(src.Spec.Networking.Labels)
		dst.Spec.Server.CheServerIngress.Annotations = utils.CloneMap(src.Spec.Networking.Annotations)
		delete(dst.Spec.Server.CheServerIngress.Annotations, "kubernetes.io/ingress.class")
	}

	if src.Spec.Networking.Auth.Gateway.Deployment != nil {
		for _, c := range src.Spec.Networking.Auth.Gateway.Deployment.Containers {
			switch c.Name {
			case constants.GatewayContainerName:
				dst.Spec.Server.SingleHostGatewayImage = c.Image
			case constants.GatewayConfigSideCarContainerName:
				dst.Spec.Server.SingleHostGatewayConfigSidecarImage = c.Image
			}
		}
	}

	dst.Spec.Server.SingleHostGatewayConfigMapLabels = utils.CloneMap(src.Spec.Networking.Auth.Gateway.ConfigLabels)

	trustStoreConfigMap, err := findTrustStoreConfigMap(src.Namespace)
	if err != nil {
		return err
	} else {
		dst.Spec.Server.ServerTrustStoreConfigMapName = trustStoreConfigMap
	}

	if src.Spec.DevEnvironments.TrustedCerts != nil && src.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName != "" {
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
	dst.Spec.Server.OpenVSXRegistryURL = src.Spec.Components.PluginRegistry.OpenVSXURL
	dst.Spec.Server.ExternalPluginRegistry = src.Spec.Components.PluginRegistry.DisableInternalRegistry

	if src.Spec.Components.PluginRegistry.DisableInternalRegistry {
		if len(src.Spec.Components.PluginRegistry.ExternalPluginRegistries) != 0 {
			dst.Spec.Server.PluginRegistryUrl = src.Spec.Components.PluginRegistry.ExternalPluginRegistries[0].Url
		}
	}

	if src.Spec.Components.PluginRegistry.Deployment != nil {
		if len(src.Spec.Components.PluginRegistry.Deployment.Containers) != 0 {
			dst.Spec.Server.PluginRegistryEnv = src.Spec.Components.PluginRegistry.Deployment.Containers[0].Env
			dst.Spec.Server.PluginRegistryImage = src.Spec.Components.PluginRegistry.Deployment.Containers[0].Image
			dst.Spec.Server.PluginRegistryPullPolicy = src.Spec.Components.PluginRegistry.Deployment.Containers[0].ImagePullPolicy

			if src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources != nil {
				if src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Requests != nil {
					dst.Spec.Server.PluginRegistryMemoryRequest = resource2String(src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Requests.Memory)
					dst.Spec.Server.PluginRegistryCpuRequest = resource2String(src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Requests.Cpu)
				}
				if src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Limits != nil {
					dst.Spec.Server.PluginRegistryMemoryLimit = resource2String(src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Limits.Memory)
					dst.Spec.Server.PluginRegistryCpuLimit = resource2String(src.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Limits.Cpu)
				}
			}
		}
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server_DevfileRegistry(src *chev2.CheCluster) error {
	dst.Spec.Server.ExternalDevfileRegistry = src.Spec.Components.DevfileRegistry.DisableInternalRegistry

	for _, r := range src.Spec.Components.DevfileRegistry.ExternalDevfileRegistries {
		dst.Spec.Server.ExternalDevfileRegistries = append(dst.Spec.Server.ExternalDevfileRegistries,
			ExternalDevfileRegistries{
				Url: r.Url,
			})
	}

	if src.Spec.Components.DevfileRegistry.Deployment != nil {
		if len(src.Spec.Components.DevfileRegistry.Deployment.Containers) != 0 {
			dst.Spec.Server.DevfileRegistryEnv = src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Env
			dst.Spec.Server.DevfileRegistryImage = src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Image
			dst.Spec.Server.DevfileRegistryPullPolicy = src.Spec.Components.DevfileRegistry.Deployment.Containers[0].ImagePullPolicy

			if src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources != nil {
				if src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Requests != nil {
					dst.Spec.Server.DevfileRegistryMemoryRequest = resource2String(src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Requests.Memory)
					dst.Spec.Server.DevfileRegistryCpuRequest = resource2String(src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Requests.Cpu)
				}
				if src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Limits != nil {
					dst.Spec.Server.DevfileRegistryMemoryLimit = resource2String(src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Limits.Memory)
					dst.Spec.Server.DevfileRegistryCpuLimit = resource2String(src.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Limits.Cpu)
				}
			}
		}
	}

	return nil
}

func (dst *CheCluster) convertFrom_Server_Dashboard(src *chev2.CheCluster) error {
	if src.Spec.Components.Dashboard.Deployment != nil {
		if len(src.Spec.Components.Dashboard.Deployment.Containers) != 0 {
			dst.Spec.Server.DashboardEnv = src.Spec.Components.Dashboard.Deployment.Containers[0].Env
			dst.Spec.Server.DashboardImage = src.Spec.Components.Dashboard.Deployment.Containers[0].Image
			dst.Spec.Server.DashboardImagePullPolicy = string(src.Spec.Components.Dashboard.Deployment.Containers[0].ImagePullPolicy)
			if src.Spec.Components.Dashboard.Deployment.Containers[0].Resources != nil {
				if src.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Requests != nil {
					dst.Spec.Server.DashboardMemoryRequest = resource2String(src.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Requests.Memory)
					dst.Spec.Server.DashboardCpuRequest = resource2String(src.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Requests.Cpu)
				}
				if src.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Limits != nil {
					dst.Spec.Server.DashboardMemoryLimit = resource2String(src.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Limits.Memory)
					dst.Spec.Server.DashboardCpuLimit = resource2String(src.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Limits.Cpu)
				}
			}
		}
	}

	return nil
}

func (dst *CheCluster) convertFrom_K8s(src *chev2.CheCluster) error {
	if src.Spec.Components.CheServer.Deployment != nil {
		if src.Spec.Components.CheServer.Deployment.SecurityContext != nil {
			if src.Spec.Components.CheServer.Deployment.SecurityContext.RunAsUser != nil {
				dst.Spec.K8s.SecurityContextRunAsUser = strconv.FormatInt(*src.Spec.Components.CheServer.Deployment.SecurityContext.RunAsUser, 10)
			}
			if src.Spec.Components.CheServer.Deployment.SecurityContext.FsGroup != nil {
				dst.Spec.K8s.SecurityContextFsGroup = strconv.FormatInt(*src.Spec.Components.CheServer.Deployment.SecurityContext.FsGroup, 10)
			}
		}
	}

	if !infrastructure.IsOpenShift() {
		dst.Spec.K8s.IngressDomain = src.Spec.Networking.Domain
		dst.Spec.K8s.TlsSecretName = src.Spec.Networking.TlsSecretName
		dst.Spec.K8s.IngressClass = src.Spec.Networking.Annotations["kubernetes.io/ingress.class"]
	}

	return nil
}

func (dst *CheCluster) convertFrom_Auth(src *chev2.CheCluster) error {
	dst.Spec.Auth.IdentityProviderURL = src.Spec.Networking.Auth.IdentityProviderURL
	dst.Spec.Auth.OAuthClientName = src.Spec.Networking.Auth.OAuthClientName
	dst.Spec.Auth.OAuthSecret = src.Spec.Networking.Auth.OAuthSecret
	dst.Spec.Auth.OAuthScope = src.Spec.Networking.Auth.OAuthScope
	dst.Spec.Auth.IdentityToken = src.Spec.Networking.Auth.IdentityToken

	if src.Spec.Networking.Auth.Gateway.Deployment != nil {
		for _, c := range src.Spec.Networking.Auth.Gateway.Deployment.Containers {
			switch c.Name {
			case constants.GatewayAuthenticationContainerName:
				dst.Spec.Auth.GatewayAuthenticationSidecarImage = c.Image
				dst.Spec.Auth.GatewayOAuthProxyEnv = c.Env
			case constants.GatewayAuthorizationContainerName:
				dst.Spec.Auth.GatewayAuthorizationSidecarImage = c.Image
				dst.Spec.Auth.GatewayKubeRbacProxyEnv = c.Env
			case constants.GatewayConfigSideCarContainerName:
				dst.Spec.Auth.GatewayConfigBumpEnv = c.Env
			case constants.GatewayContainerName:
				dst.Spec.Auth.GatewayEnv = c.Env
			}
		}
	}

	return nil
}

func (dst *CheCluster) convertFrom_DevWorkspace(src *chev2.CheCluster) error {
	if src.Spec.DevEnvironments.MaxNumberOfRunningWorkspacesPerUser != nil {
		dst.Spec.DevWorkspace.RunningLimit = strconv.FormatInt(*src.Spec.DevEnvironments.MaxNumberOfRunningWorkspacesPerUser, 10)
	}
	dst.Spec.DevWorkspace.SecondsOfInactivityBeforeIdling = src.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling
	dst.Spec.DevWorkspace.SecondsOfRunBeforeIdling = src.Spec.DevEnvironments.SecondsOfRunBeforeIdling
	dst.Spec.DevWorkspace.Enable = true
	return nil
}

func (dst *CheCluster) convertFrom_ImagePuller(src *chev2.CheCluster) error {
	dst.Spec.ImagePuller.Enable = src.Spec.Components.ImagePuller.Enable
	dst.Spec.ImagePuller.Spec = src.Spec.Components.ImagePuller.Spec

	return nil
}

func (dst *CheCluster) convertFrom_Metrics(src *chev2.CheCluster) error {
	dst.Spec.Metrics.Enable = src.Spec.Components.Metrics.Enable

	return nil
}

func (dst *CheCluster) convertFrom_Dashboard(src *chev2.CheCluster) error {
	if src.Spec.Components.Dashboard.HeaderMessage != nil {
		dst.Spec.Dashboard.Warning = src.Spec.Components.Dashboard.HeaderMessage.Text
	}

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

	if src.Spec.DevEnvironments.TrustedCerts != nil && src.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName != "" {
		dst.Status.GitServerTLSCertificateConfigMapName = src.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName
	}

	return nil
}

func (dst *CheCluster) convertFrom_Storage(src *chev2.CheCluster) error {
	dst.Spec.Storage.PvcStrategy = src.Spec.DevEnvironments.Storage.PvcStrategy
	if src.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig != nil {
		dst.Spec.Storage.PvcClaimSize = src.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize
		dst.Spec.Storage.WorkspacePVCStorageClassName = src.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass
	}

	if src.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig != nil {
		dst.Spec.Storage.PerWorkspaceStrategyPvcClaimSize = src.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize
		dst.Spec.Storage.PerWorkspaceStrategyPVCStorageClassName = src.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass
	}

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

func resource2String(resource *resource.Quantity) string {
	if resource == nil || resource.IsZero() {
		return ""
	}
	return resource.String()
}
