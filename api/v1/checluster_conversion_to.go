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
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/eclipse-che/che-operator/pkg/common/utils"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

var (
	logger = ctrl.Log.WithName("conversion")
)

func (src *CheCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*chev2.CheCluster)
	dst.ObjectMeta = src.ObjectMeta

	if err := src.convertTo_Components(dst); err != nil {
		return err
	}

	if err := src.convertTo_Networking(dst); err != nil {
		return err
	}

	if err := src.convertTo_DevEnvironments(dst); err != nil {
		return err
	}

	if err := src.convertTo_ContainerRegistry(dst); err != nil {
		return err
	}

	if err := src.convertTo_GitServices(dst); err != nil {
		return err
	}

	if err := src.convertTo_Status(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_GitServices(dst *chev2.CheCluster) error {
	for _, github := range src.Spec.GitServices.GitHub {
		dst.Spec.GitServices.GitHub = append(
			dst.Spec.GitServices.GitHub,
			chev2.GitHubService{
				SecretName: github.SecretName,
			})
	}

	for _, gitlab := range src.Spec.GitServices.GitLab {
		dst.Spec.GitServices.GitLab = append(
			dst.Spec.GitServices.GitLab,
			chev2.GitLabService{
				SecretName: gitlab.SecretName,
			})
	}

	for _, bitbucket := range src.Spec.GitServices.BitBucket {
		dst.Spec.GitServices.BitBucket = append(
			dst.Spec.GitServices.BitBucket,
			chev2.BitBucketService{
				SecretName: bitbucket.SecretName,
			})
	}

	return nil
}

func (src *CheCluster) convertTo_Status(dst *chev2.CheCluster) error {
	dst.Status.CheURL = src.Status.CheURL
	dst.Status.CheVersion = src.Status.CheVersion
	dst.Status.DevfileRegistryURL = src.Status.DevfileRegistryURL
	dst.Status.PluginRegistryURL = src.Status.PluginRegistryURL
	dst.Status.Message = src.Status.Message
	dst.Status.Reason = src.Status.Reason
	dst.Status.GatewayPhase = chev2.GatewayPhase(src.Status.DevworkspaceStatus.GatewayPhase)
	dst.Status.WorkspaceBaseDomain = src.Status.DevworkspaceStatus.WorkspaceBaseDomain

	switch src.Status.CheClusterRunning {
	case "Available":
		dst.Status.ChePhase = chev2.ClusterPhaseActive
	case "Unavailable":
		dst.Status.ChePhase = chev2.ClusterPhaseInactive
	case "Available, Rolling Update in Progress":
		dst.Status.ChePhase = chev2.RollingUpdate
	}

	return nil
}

func (src *CheCluster) convertTo_ContainerRegistry(dst *chev2.CheCluster) error {
	dst.Spec.ContainerRegistry.Hostname = src.Spec.Server.AirGapContainerRegistryHostname
	dst.Spec.ContainerRegistry.Organization = src.Spec.Server.AirGapContainerRegistryOrganization

	return nil
}

func (src *CheCluster) convertTo_DevEnvironments(dst *chev2.CheCluster) error {
	if src.Spec.Server.GitSelfSignedCert {
		if src.Status.GitServerTLSCertificateConfigMapName != "" {
			dst.Spec.DevEnvironments.TrustedCerts = &chev2.TrustedCerts{
				GitTrustedCertsConfigMapName: src.Status.GitServerTLSCertificateConfigMapName,
			}
		} else {
			dst.Spec.DevEnvironments.TrustedCerts = &chev2.TrustedCerts{
				GitTrustedCertsConfigMapName: constants.DefaultGitSelfSignedCertsConfigMapName,
			}
		}
	}

	dst.Spec.DevEnvironments.DefaultNamespace.Template = src.Spec.Server.WorkspaceNamespaceDefault
	dst.Spec.DevEnvironments.DefaultNamespace.AutoProvision = src.Spec.Server.AllowAutoProvisionUserNamespace
	dst.Spec.DevEnvironments.NodeSelector = utils.CloneMap(src.Spec.Server.WorkspacePodNodeSelector)

	for _, v := range src.Spec.Server.WorkspacePodTolerations {
		dst.Spec.DevEnvironments.Tolerations = append(dst.Spec.DevEnvironments.Tolerations, v)
	}

	for _, p := range src.Spec.Server.WorkspacesDefaultPlugins {
		dst.Spec.DevEnvironments.DefaultPlugins = append(dst.Spec.DevEnvironments.DefaultPlugins,
			chev2.WorkspaceDefaultPlugins{
				Editor:  p.Editor,
				Plugins: p.Plugins,
			})
	}

	dst.Spec.DevEnvironments.DefaultEditor = src.Spec.Server.WorkspaceDefaultEditor
	dst.Spec.DevEnvironments.DefaultComponents = src.Spec.Server.WorkspaceDefaultComponents
	dst.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling = src.Spec.DevWorkspace.SecondsOfInactivityBeforeIdling
	dst.Spec.DevEnvironments.SecondsOfRunBeforeIdling = src.Spec.DevWorkspace.SecondsOfRunBeforeIdling

	if err := src.convertTo_Workspaces_Storage(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Workspaces_Storage(dst *chev2.CheCluster) error {
	dst.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig = toCheV2Pvc(src.Spec.Storage.PvcClaimSize, src.Spec.Storage.WorkspacePVCStorageClassName)
	dst.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig = toCheV2Pvc(src.Spec.Storage.PerWorkspaceStrategyPvcClaimSize, src.Spec.Storage.PerWorkspaceStrategyPVCStorageClassName)
	dst.Spec.DevEnvironments.Storage.PvcStrategy = src.Spec.Storage.PvcStrategy
	return nil
}

func (src *CheCluster) convertTo_Networking(dst *chev2.CheCluster) error {
	if infrastructure.IsOpenShift() {
		dst.Spec.Networking = chev2.CheClusterSpecNetworking{
			Labels:        utils.ParseMap(src.Spec.Server.CheServerRoute.Labels),
			Annotations:   utils.CloneMap(src.Spec.Server.CheServerRoute.Annotations),
			Hostname:      src.Spec.Server.CheHost,
			Domain:        src.Spec.Server.CheServerRoute.Domain,
			TlsSecretName: src.Spec.Server.CheHostTLSSecret,
		}
	} else {
		dst.Spec.Networking = chev2.CheClusterSpecNetworking{
			Labels:   utils.ParseMap(src.Spec.Server.CheServerIngress.Labels),
			Domain:   src.Spec.K8s.IngressDomain,
			Hostname: src.Spec.Server.CheHost,
		}

		if src.Spec.Server.CheHostTLSSecret != "" {
			dst.Spec.Networking.TlsSecretName = src.Spec.Server.CheHostTLSSecret
		} else {
			dst.Spec.Networking.TlsSecretName = src.Spec.K8s.TlsSecretName
		}

		if src.Spec.K8s.IngressClass != "" {
			dst.Spec.Networking.Annotations = map[string]string{"kubernetes.io/ingress.class": src.Spec.K8s.IngressClass}
		}

		if len(dst.Spec.Networking.Annotations) > 0 || len(src.Spec.Server.CheServerIngress.Annotations) > 0 {
			dst.Spec.Networking.Annotations = labels.Merge(dst.Spec.Networking.Annotations, src.Spec.Server.CheServerIngress.Annotations)
		}
	}

	if err := src.convertTo_Networking_Auth(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Networking_Auth(dst *chev2.CheCluster) error {
	dst.Spec.Networking.Auth.IdentityProviderURL = src.Spec.Auth.IdentityProviderURL
	dst.Spec.Networking.Auth.OAuthClientName = src.Spec.Auth.OAuthClientName
	dst.Spec.Networking.Auth.OAuthSecret = src.Spec.Auth.OAuthSecret
	dst.Spec.Networking.Auth.OAuthScope = src.Spec.Auth.OAuthScope
	dst.Spec.Networking.Auth.IdentityToken = src.Spec.Auth.IdentityToken

	if err := src.convertTo_Networking_Auth_Gateway(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Networking_Auth_Gateway(dst *chev2.CheCluster) error {
	dst.Spec.Networking.Auth.Gateway.ConfigLabels = utils.CloneMap(src.Spec.Server.SingleHostGatewayConfigMapLabels)

	containers2add := append([]*chev2.Container{},
		toCheV2ContainerWithImageAndEnv(constants.GatewayContainerName, src.Spec.Server.SingleHostGatewayImage, src.Spec.Auth.GatewayEnv),
		toCheV2ContainerWithImageAndEnv(constants.GatewayConfigSideCarContainerName, src.Spec.Server.SingleHostGatewayConfigSidecarImage, src.Spec.Auth.GatewayConfigBumpEnv),
		toCheV2ContainerWithImageAndEnv(constants.GatewayAuthenticationContainerName, src.Spec.Auth.GatewayAuthenticationSidecarImage, src.Spec.Auth.GatewayOAuthProxyEnv),
		toCheV2ContainerWithImageAndEnv(constants.GatewayAuthorizationContainerName, src.Spec.Auth.GatewayAuthorizationSidecarImage, src.Spec.Auth.GatewayKubeRbacProxyEnv))

	for _, container := range containers2add {
		if container != nil {
			if dst.Spec.Networking.Auth.Gateway.Deployment == nil {
				dst.Spec.Networking.Auth.Gateway.Deployment = &chev2.Deployment{}
			}
			dst.Spec.Networking.Auth.Gateway.Deployment.Containers = append(dst.Spec.Networking.Auth.Gateway.Deployment.Containers, *container)
		}
	}
	return nil
}

func (src *CheCluster) convertTo_Components(dst *chev2.CheCluster) error {
	if err := src.convertTo_Components_Dashboard(dst); err != nil {
		return err
	}

	if err := src.convertTo_Components_DevfileRegistry(dst); err != nil {
		return err
	}

	if err := src.convertTo_Components_PluginRegistry(dst); err != nil {
		return err
	}

	if err := src.convertTo_Components_CheServer(dst); err != nil {
		return err
	}

	if err := src.convertTo_Components_Metrics(dst); err != nil {
		return err
	}

	if err := src.convertTo_Components_ImagePuller(dst); err != nil {
		return err
	}

	if err := src.convertTo_Components_DevWorkspace(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Components_DevWorkspace(dst *chev2.CheCluster) error {
	if src.Spec.DevWorkspace.RunningLimit != "" {
		runningLimit, err := strconv.ParseInt(src.Spec.DevWorkspace.RunningLimit, 10, 64)
		dst.Spec.DevEnvironments.MaxNumberOfRunningWorkspacesPerUser = pointer.Int64Ptr(runningLimit)
		return err
	}
	return nil
}

func (src *CheCluster) convertTo_Components_ImagePuller(dst *chev2.CheCluster) error {
	dst.Spec.Components.ImagePuller.Enable = src.Spec.ImagePuller.Enable
	dst.Spec.Components.ImagePuller.Spec = src.Spec.ImagePuller.Spec
	return nil
}

func (src *CheCluster) convertTo_Components_Metrics(dst *chev2.CheCluster) error {
	dst.Spec.Components.Metrics.Enable = src.Spec.Metrics.Enable
	return nil
}

func (src *CheCluster) convertTo_Components_CheServer(dst *chev2.CheCluster) error {
	dst.Spec.Components.CheServer.ExtraProperties = utils.CloneMap(src.Spec.Server.CustomCheProperties)
	dst.Spec.Components.CheServer.LogLevel = src.Spec.Server.CheLogLevel
	if src.Spec.Server.CheClusterRoles != "" {
		dst.Spec.Components.CheServer.ClusterRoles = strings.Split(src.Spec.Server.CheClusterRoles, ",")
	}

	if src.Spec.Server.CheDebug != "" {
		debug, err := strconv.ParseBool(src.Spec.Server.CheDebug)
		if err != nil {
			return err
		} else {
			dst.Spec.Components.CheServer.Debug = pointer.BoolPtr(debug)
		}
	}

	if src.Spec.Server.ProxyURL != "" || src.Spec.Server.ProxyPort != "" || src.Spec.Server.ProxySecret != "" {
		dst.Spec.Components.CheServer.Proxy = &chev2.Proxy{
			Url:                   src.Spec.Server.ProxyURL,
			Port:                  src.Spec.Server.ProxyPort,
			CredentialsSecretName: src.Spec.Server.ProxySecret,
		}
	}

	if src.Spec.Server.NonProxyHosts != "" {
		if dst.Spec.Components.CheServer.Proxy == nil {
			dst.Spec.Components.CheServer.Proxy = &chev2.Proxy{}
		}
		dst.Spec.Components.CheServer.Proxy.NonProxyHosts = strings.Split(src.Spec.Server.NonProxyHosts, "|")
	}

	if src.Spec.Server.ProxySecret == "" && src.Spec.Server.ProxyUser != "" && src.Spec.Server.ProxyPassword != "" {
		if err := createCredentialsSecret(
			src.Spec.Server.ProxyUser,
			src.Spec.Server.ProxyPassword,
			constants.DefaultProxyCredentialsSecret,
			src.ObjectMeta.Namespace); err != nil {
			return err
		}

		if dst.Spec.Components.CheServer.Proxy == nil {
			dst.Spec.Components.CheServer.Proxy = &chev2.Proxy{}
		}
		dst.Spec.Components.CheServer.Proxy.CredentialsSecretName = constants.DefaultProxyCredentialsSecret
	}

	runAsUser, fsGroup, err := parseSecurityContext(src)
	if err != nil {
		return err
	}

	dst.Spec.Components.CheServer.Deployment = toCheV2Deployment(
		defaults.GetCheFlavor(),
		map[bool]string{true: src.Spec.Server.CheImage + ":" + src.Spec.Server.CheImageTag, false: ""}[src.Spec.Server.CheImage != ""],
		src.Spec.Server.CheImagePullPolicy,
		src.Spec.Server.ServerMemoryRequest,
		src.Spec.Server.ServerMemoryLimit,
		src.Spec.Server.ServerCpuRequest,
		src.Spec.Server.ServerCpuLimit,
		fsGroup,
		runAsUser,
		src.Spec.Server.CheServerEnv,
	)

	if src.Spec.Server.ServerTrustStoreConfigMapName != "" {
		if err := renameTrustStoreConfigMapToDefault(src.Spec.Server.ServerTrustStoreConfigMapName, src.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func (src *CheCluster) convertTo_Components_PluginRegistry(dst *chev2.CheCluster) error {
	dst.Spec.Components.PluginRegistry.OpenVSXURL = src.Spec.Server.OpenVSXRegistryURL
	dst.Spec.Components.PluginRegistry.DisableInternalRegistry = src.Spec.Server.ExternalPluginRegistry

	if dst.Spec.Components.PluginRegistry.DisableInternalRegistry {
		dst.Spec.Components.PluginRegistry.ExternalPluginRegistries = []chev2.ExternalPluginRegistry{
			{
				Url: src.Spec.Server.PluginRegistryUrl,
			},
		}
	}

	dst.Spec.Components.PluginRegistry.Deployment = toCheV2Deployment(
		constants.PluginRegistryName,
		src.Spec.Server.PluginRegistryImage,
		src.Spec.Server.PluginRegistryPullPolicy,
		src.Spec.Server.PluginRegistryMemoryRequest,
		src.Spec.Server.PluginRegistryMemoryLimit,
		src.Spec.Server.PluginRegistryCpuRequest,
		src.Spec.Server.PluginRegistryCpuLimit,
		nil,
		nil,
		src.Spec.Server.PluginRegistryEnv,
	)

	return nil
}

func (src *CheCluster) convertTo_Components_DevfileRegistry(dst *chev2.CheCluster) error {
	dst.Spec.Components.DevfileRegistry.DisableInternalRegistry = src.Spec.Server.ExternalDevfileRegistry

	for _, r := range src.Spec.Server.ExternalDevfileRegistries {
		dst.Spec.Components.DevfileRegistry.ExternalDevfileRegistries = append(dst.Spec.Components.DevfileRegistry.ExternalDevfileRegistries,
			chev2.ExternalDevfileRegistry{
				Url: r.Url,
			})
	}

	dst.Spec.Components.DevfileRegistry.Deployment = toCheV2Deployment(
		constants.DevfileRegistryName,
		src.Spec.Server.DevfileRegistryImage,
		src.Spec.Server.DevfileRegistryPullPolicy,
		src.Spec.Server.DevfileRegistryMemoryRequest,
		src.Spec.Server.DevfileRegistryMemoryLimit,
		src.Spec.Server.DevfileRegistryCpuRequest,
		src.Spec.Server.DevfileRegistryCpuLimit,
		nil,
		nil,
		src.Spec.Server.DevfileRegistryEnv,
	)

	return nil
}

func (src *CheCluster) convertTo_Components_Dashboard(dst *chev2.CheCluster) error {
	runAsUser, fsGroup, err := parseSecurityContext(src)
	if err != nil {
		return err
	}

	dst.Spec.Components.Dashboard.Deployment = toCheV2Deployment(
		defaults.GetCheFlavor()+"-dashboard",
		src.Spec.Server.DashboardImage,
		corev1.PullPolicy(src.Spec.Server.DashboardImagePullPolicy),
		src.Spec.Server.DashboardMemoryRequest,
		src.Spec.Server.DashboardMemoryLimit,
		src.Spec.Server.DashboardCpuRequest,
		src.Spec.Server.DashboardCpuLimit,
		fsGroup,
		runAsUser,
		src.Spec.Server.DashboardEnv,
	)

	if src.Spec.Dashboard.Warning != "" {
		dst.Spec.Components.Dashboard.HeaderMessage = &chev2.DashboardHeaderMessage{
			Text: src.Spec.Dashboard.Warning,
			Show: src.Spec.Dashboard.Warning != "",
		}
	}

	return nil
}

func parseSecurityContext(cheClusterV1 *CheCluster) (*int64, *int64, error) {
	var runAsUser *int64 = nil
	if cheClusterV1.Spec.K8s.SecurityContextRunAsUser != "" {
		intValue, err := strconv.ParseInt(cheClusterV1.Spec.K8s.SecurityContextRunAsUser, 10, 64)
		if err != nil {
			return nil, nil, err
		}

		runAsUser = pointer.Int64Ptr(intValue)
	}

	var fsGroup *int64 = nil
	if cheClusterV1.Spec.K8s.SecurityContextFsGroup != "" {
		intValue, err := strconv.ParseInt(cheClusterV1.Spec.K8s.SecurityContextFsGroup, 10, 64)
		if err != nil {
			return nil, nil, err
		}
		fsGroup = pointer.Int64Ptr(intValue)
	}

	return runAsUser, fsGroup, nil
}

// Create a secret with a user's credentials
// Username and password are stored in `user` and `password` fields correspondingly.
func createCredentialsSecret(username string, password string, secretName string, namespace string) error {
	k8sHelper := k8shelper.New()

	_, err := k8sHelper.GetClientset().CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err == nil {
		// Credentials secret already exists, we can't proceed
		return fmt.Errorf("secret %s already exists", secretName)
	}

	secret := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "che.eclipse.org",
			},
		},
		Data: map[string][]byte{
			"user":     []byte(username),
			"password": []byte(password),
		},
	}

	if _, err := k8sHelper.GetClientset().CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return err
	}

	logger.Info("Credentials secret '" + secretName + "' with created.")
	return nil
}

// Convert `server.ServerTrustStoreConfigMapName` field from API V1 to API V2
// Since we API V2 does not have `server.ServerTrustStoreConfigMapName` field, we need to create
// the same ConfigMap but with a default name to be correctly handled by a controller.
func renameTrustStoreConfigMapToDefault(trustStoreConfigMapName string, namespace string) error {
	if trustStoreConfigMapName == constants.DefaultServerTrustStoreConfigMapName {
		// Already in default name
		return nil
	}

	k8sHelper := k8shelper.New()

	_, err := k8sHelper.GetClientset().CoreV1().ConfigMaps(namespace).Get(context.TODO(), constants.DefaultServerTrustStoreConfigMapName, metav1.GetOptions{})
	if err == nil {
		// ConfigMap with a default name already exists, we can't proceed
		return fmt.Errorf("TrustStore ConfigMap %s already exists", constants.DefaultServerTrustStoreConfigMapName)
	}

	existedTrustStoreConfigMap, err := k8sHelper.GetClientset().CoreV1().ConfigMaps(namespace).Get(context.TODO(), trustStoreConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap not found, nothing to rename
			return nil
		}
		return err
	}

	// must have labels
	newTrustStoreConfigMapLabels := map[string]string{
		"app.kubernetes.io/part-of":   "che.eclipse.org",
		"app.kubernetes.io/component": "ca-bundle",
	}

	newTrustStoreConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultServerTrustStoreConfigMapName,
			Namespace: namespace,
			Labels:    labels.Merge(newTrustStoreConfigMapLabels, existedTrustStoreConfigMap.Labels),
		},
		Data: existedTrustStoreConfigMap.Data,
	}

	// Create TrustStore ConfigMap with a default name
	if _, err = k8sHelper.GetClientset().CoreV1().ConfigMaps(namespace).Create(context.TODO(), newTrustStoreConfigMap, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Delete legacy TrustStore ConfigMap
	if err = k8sHelper.GetClientset().CoreV1().ConfigMaps(namespace).Delete(context.TODO(), trustStoreConfigMapName, metav1.DeleteOptions{}); err != nil {
		return err
	}

	logger.Info("TrustStore ConfigMap '" + constants.DefaultServerTrustStoreConfigMapName + "' created.")
	return nil
}

func toCheV2ContainerWithImageAndEnv(
	name string,
	image string,
	env []corev1.EnvVar) *chev2.Container {

	var container *chev2.Container
	if image != "" {
		container = &chev2.Container{
			Name:  name,
			Image: image,
		}
	}

	if len(env) != 0 {
		if container == nil {
			container = &chev2.Container{
				Name: name,
			}
		}
		container.Env = env
	}

	return container
}

func toCheV2Deployment(
	name string,
	image string,
	imagePullPolicy corev1.PullPolicy,
	memoryRequest string,
	memoryLimit string,
	cpuRequest string,
	cpuLimit string,
	fsGroup *int64,
	runAsUser *int64,
	env []corev1.EnvVar) *chev2.Deployment {

	var deployment *chev2.Deployment
	var container *chev2.Container
	var resources *chev2.ResourceRequirements

	if image != "" || imagePullPolicy != "" {
		container = &chev2.Container{
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
		}
	}

	if memoryRequest != "" || cpuRequest != "" {
		if resources == nil {
			resources = &chev2.ResourceRequirements{}
		}
		resources.Requests = &chev2.ResourceList{}

		if memoryRequest != "" {
			m := resource.MustParse(memoryRequest)
			resources.Requests.Memory = &m
		}
		if cpuRequest != "" {
			c := resource.MustParse(cpuRequest)
			resources.Requests.Cpu = &c
		}
	}

	if memoryLimit != "" || cpuLimit != "" {
		if resources == nil {
			resources = &chev2.ResourceRequirements{}
		}
		resources.Limits = &chev2.ResourceList{}

		if memoryLimit != "" {
			m := resource.MustParse(memoryLimit)
			resources.Limits.Memory = &m
		}
		if cpuLimit != "" {
			c := resource.MustParse(cpuLimit)
			resources.Limits.Cpu = &c
		}
	}

	if resources != nil {
		if container == nil {
			container = &chev2.Container{}
		}
		container.Resources = resources
	}

	if len(env) != 0 {
		if container == nil {
			container = &chev2.Container{}
		}
		container.Env = env
	}

	if runAsUser != nil || fsGroup != nil {
		if deployment == nil {
			deployment = &chev2.Deployment{}
		}

		deployment.SecurityContext = &chev2.PodSecurityContext{
			RunAsUser: runAsUser,
			FsGroup:   fsGroup,
		}
	}

	if container != nil {
		if deployment == nil {
			deployment = &chev2.Deployment{}
		}

		container.Name = name
		deployment.Containers = []chev2.Container{*container}
	}

	return deployment
}

func toCheV2Pvc(claimSize string, storageClass string) *chev2.PVC {
	if claimSize != "" || storageClass != "" {
		return &chev2.PVC{
			ClaimSize:    claimSize,
			StorageClass: storageClass,
		}
	}

	return nil
}
