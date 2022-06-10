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
	"reflect"
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

	if err := src.convertTo_Status(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Status(dst *chev2.CheCluster) error {
	dst.Status.PostgresVersion = src.Spec.Database.PostgresVersion
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
			dst.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName = src.Status.GitServerTLSCertificateConfigMapName
		} else {
			dst.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName = constants.DefaultGitSelfSignedCertsConfigMapName
		}
	}

	dst.Spec.DevEnvironments.DefaultNamespace.Template = src.Spec.Server.WorkspaceNamespaceDefault
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

	if err := src.convertTo_Workspaces_Storage(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Workspaces_Storage(dst *chev2.CheCluster) error {
	dst.Spec.DevEnvironments.Storage.Pvc = chev2.PVC{
		ClaimSize:    src.Spec.Storage.PvcClaimSize,
		StorageClass: src.Spec.Storage.WorkspacePVCStorageClassName,
	}
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

	if err := src.convertTo_Networking_Auth_Gateway(dst); err != nil {
		return err
	}

	return nil
}

func (src *CheCluster) convertTo_Networking_Auth_Gateway(dst *chev2.CheCluster) error {
	dst.Spec.Networking.Auth.Gateway.ConfigLabels = utils.CloneMap(src.Spec.Server.SingleHostGatewayConfigMapLabels)

	if src.Spec.Server.SingleHostGatewayImage != "" {
		dst.Spec.Networking.Auth.Gateway.Deployment.Containers = append(
			dst.Spec.Networking.Auth.Gateway.Deployment.Containers,
			chev2.Container{
				Name:  constants.GatewayContainerName,
				Image: src.Spec.Server.SingleHostGatewayImage,
			},
		)
	}

	if src.Spec.Server.SingleHostGatewayConfigSidecarImage != "" {
		dst.Spec.Networking.Auth.Gateway.Deployment.Containers = append(
			dst.Spec.Networking.Auth.Gateway.Deployment.Containers,
			chev2.Container{
				Name:  constants.GatewayConfigSideCarContainerName,
				Image: src.Spec.Server.SingleHostGatewayConfigSidecarImage,
			},
		)
	}

	if src.Spec.Auth.GatewayAuthenticationSidecarImage != "" {
		dst.Spec.Networking.Auth.Gateway.Deployment.Containers = append(
			dst.Spec.Networking.Auth.Gateway.Deployment.Containers,
			chev2.Container{
				Name:  constants.GatewayAuthenticationContainerName,
				Image: src.Spec.Auth.GatewayAuthenticationSidecarImage,
			},
		)
	}

	if src.Spec.Auth.GatewayAuthorizationSidecarImage != "" {
		dst.Spec.Networking.Auth.Gateway.Deployment.Containers = append(
			dst.Spec.Networking.Auth.Gateway.Deployment.Containers,
			chev2.Container{
				Name:  constants.GatewayAuthorizationContainerName,
				Image: src.Spec.Auth.GatewayAuthorizationSidecarImage,
			},
		)
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

	if err := src.convertTo_Components_Database(dst); err != nil {
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
	if src.Spec.DevWorkspace.ControllerImage != "" {
		dst.Spec.Components.DevWorkspace.Deployment = chev2.Deployment{
			Containers: []chev2.Container{
				{
					Name:  constants.DevWorkspaceController,
					Image: src.Spec.DevWorkspace.ControllerImage,
				},
			},
		}
	}
	dst.Spec.Components.DevWorkspace.RunningLimit = src.Spec.DevWorkspace.RunningLimit

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

	dst.Spec.Components.CheServer.Proxy = chev2.Proxy{
		Url:                   src.Spec.Server.ProxyURL,
		Port:                  src.Spec.Server.ProxyPort,
		CredentialsSecretName: src.Spec.Server.ProxySecret,
	}
	if src.Spec.Server.NonProxyHosts != "" {
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
	)

	if src.Spec.Server.ServerTrustStoreConfigMapName != "" {
		if err := renameTrustStoreConfigMapToDefault(src.Spec.Server.ServerTrustStoreConfigMapName, src.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func (src *CheCluster) convertTo_Components_PluginRegistry(dst *chev2.CheCluster) error {
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
	)

	return nil
}

func (src *CheCluster) convertTo_Components_Database(dst *chev2.CheCluster) error {
	dst.Spec.Components.Database.CredentialsSecretName = src.Spec.Database.ChePostgresSecret

	if src.Spec.Database.ChePostgresSecret == "" && src.Spec.Database.ChePostgresUser != "" && src.Spec.Database.ChePostgresPassword != "" {
		if err := createCredentialsSecret(
			src.Spec.Database.ChePostgresUser,
			src.Spec.Database.ChePostgresPassword,
			constants.DefaultPostgresCredentialsSecret,
			src.ObjectMeta.Namespace); err != nil {
			return err
		}
		dst.Spec.Components.Database.CredentialsSecretName = constants.DefaultPostgresCredentialsSecret
	}

	dst.Spec.Components.Database.Deployment = toCheV2Deployment(
		constants.PostgresName,
		src.Spec.Database.PostgresImage,
		src.Spec.Database.PostgresImagePullPolicy,
		src.Spec.Database.ChePostgresContainerResources.Requests.Memory,
		src.Spec.Database.ChePostgresContainerResources.Limits.Memory,
		src.Spec.Database.ChePostgresContainerResources.Requests.Cpu,
		src.Spec.Database.ChePostgresContainerResources.Limits.Cpu,
		nil,
		nil,
	)

	dst.Spec.Components.Database.ExternalDb = src.Spec.Database.ExternalDb
	dst.Spec.Components.Database.PostgresDb = src.Spec.Database.ChePostgresDb
	dst.Spec.Components.Database.PostgresHostName = src.Spec.Database.ChePostgresHostName
	dst.Spec.Components.Database.PostgresPort = src.Spec.Database.ChePostgresPort
	dst.Spec.Components.Database.Pvc = chev2.PVC{
		ClaimSize:    src.Spec.Database.PvcClaimSize,
		StorageClass: src.Spec.Storage.PostgresPVCStorageClassName,
	}

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
	)

	dst.Spec.Components.Dashboard.HeaderMessage.Text = src.Spec.Dashboard.Warning
	dst.Spec.Components.Dashboard.HeaderMessage.Show = src.Spec.Dashboard.Warning != ""

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

func toCheV2Deployment(
	name string,
	image string,
	imagePullPolicy corev1.PullPolicy,
	memoryRequest string,
	memoryLimit string,
	cpuRequest string,
	cpuLimit string,
	fsGroup *int64,
	runAsUser *int64) chev2.Deployment {

	deployment := chev2.Deployment{}

	container := chev2.Container{}
	if image != "" {
		container.Image = image
	}
	if imagePullPolicy != "" {
		container.ImagePullPolicy = imagePullPolicy
	}
	if memoryRequest != "" {
		container.Resources.Requests.Memory = resource.MustParse(memoryRequest)
	}
	if memoryLimit != "" {
		container.Resources.Limits.Memory = resource.MustParse(memoryLimit)
	}
	if cpuRequest != "" {
		container.Resources.Requests.Cpu = resource.MustParse(cpuRequest)
	}
	if cpuLimit != "" {
		container.Resources.Limits.Cpu = resource.MustParse(cpuLimit)
	}
	if !reflect.DeepEqual(container, chev2.Container{}) {
		container.Name = name
		deployment.Containers = []chev2.Container{container}
	}

	deployment.SecurityContext.RunAsUser = runAsUser
	deployment.SecurityContext.FsGroup = fsGroup

	return deployment
}
