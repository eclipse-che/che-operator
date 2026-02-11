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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

type CheConfigMap struct {
	JavaOpts                 string `json:"JAVA_OPTS"`
	CheHost                  string `json:"CHE_HOST"`
	ChePort                  string `json:"CHE_PORT"`
	CheDebugServer           string `json:"CHE_DEBUG_SERVER"`
	CheLogLevel              string `json:"CHE_LOG_LEVEL"`
	CheMetricsEnabled        string `json:"CHE_METRICS_ENABLED"`
	CheInfrastructure        string `json:"CHE_INFRASTRUCTURE_ACTIVE"`
	UserClusterRoles         string `json:"CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"`
	NamespaceDefault         string `json:"CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"`
	NamespaceCreationAllowed string `json:"CHE_INFRA_KUBERNETES_NAMESPACE_CREATION__ALLOWED"`
	Http2Disable             string `json:"HTTP2_DISABLE"`
	KubernetesLabels         string `json:"KUBERNETES_LABELS"`

	// TODO remove when keycloak codebase is removed from che-server component
	CheOIDCAuthServerUrl string `json:"CHE_OIDC_AUTH__SERVER__URL,omitempty"`
}

func (s *CheServerReconciler) syncConfigMap(ctx *chetypes.DeployContext) (bool, error) {
	data, err := s.getConfigMapData(ctx)
	if err != nil {
		return false, err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(getComponentName()),
		},
		Data: data,
	}

	err = ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		cm,
		&k8sclient.SyncOptions{DiffOpts: diffs.ConfigMapAllLabels},
	)

	return err == nil, err
}

func (s *CheServerReconciler) getConfigMapRevision(ctx *chetypes.DeployContext) (string, error) {
	cm := &corev1.ConfigMap{}

	if exist, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Namespace: ctx.CheCluster.Namespace,
			Name:      configMapName,
		},
		cm,
	); !exist {
		return "", err
	}

	return cm.ResourceVersion, nil
}

func (s *CheServerReconciler) getConfigMapData(ctx *chetypes.DeployContext) (cheEnv map[string]string, err error) {
	var cheInfrastructure string
	if infrastructure.IsOpenShift() {
		cheInfrastructure = "openshift"
	} else {
		cheInfrastructure = "kubernetes"
	}

	javaOpts := constants.DefaultJavaOpts
	if ctx.Proxy.HttpProxy != "" {
		javaOpts += deploy.GenerateProxyJavaOpts(ctx.Proxy, ctx.Proxy.NoProxy)
	}

	cheLogLevel := utils.GetValue(
		ctx.CheCluster.Spec.Components.CheServer.LogLevel,
		constants.DefaultServerLogLevel,
	)

	cheDebugServer := strconv.FormatBool(
		pointer.BoolDeref(
			ctx.CheCluster.Spec.Components.CheServer.Debug,
			constants.DefaultServerDebug,
		))

	chePort := strconv.Itoa(int(constants.DefaultServerPort))
	cheMetricsEnabled := strconv.FormatBool(ctx.CheCluster.Spec.Components.Metrics.Enable)

	namespaceDefault := ctx.CheCluster.GetDefaultNamespace()
	namespaceCreationAllowed := strconv.FormatBool(
		pointer.BoolDeref(
			ctx.CheCluster.Spec.DevEnvironments.DefaultNamespace.AutoProvision,
			constants.DefaultAutoProvision,
		))

	kubernetesLabels := labels.FormatLabels(deploy.GetLabels(defaults.GetCheFlavor()))

	data := &CheConfigMap{
		JavaOpts:                 javaOpts,
		CheHost:                  ctx.CheHost,
		ChePort:                  chePort,
		CheDebugServer:           cheDebugServer,
		CheLogLevel:              cheLogLevel,
		CheMetricsEnabled:        cheMetricsEnabled,
		CheInfrastructure:        cheInfrastructure,
		CheOIDCAuthServerUrl:     ctx.CheCluster.Spec.Networking.Auth.IdentityProviderURL,
		NamespaceDefault:         namespaceDefault,
		NamespaceCreationAllowed: namespaceCreationAllowed,
		KubernetesLabels:         kubernetesLabels,
		// Disable HTTP2 protocol.
		// Fix issue with creating config maps on the cluster https://issues.redhat.com/browse/CRW-2677
		// The root cause is in the HTTP2 protocol support of the okttp3 library that is used by fabric8.kubernetes-client that is used by che-server
		// In the past, when che-server used Java 8, HTTP1 protocol was used. Now che-sever uses Java 11
		Http2Disable: strconv.FormatBool(true),
	}

	out, err := json.Marshal(data)
	if err != nil {
		return nil, err

	}

	err = json.Unmarshal(out, &cheEnv)
	if err != nil {
		return nil, err
	}

	// override envs by extra properties
	maps.Copy(cheEnv, ctx.CheCluster.Spec.Components.CheServer.ExtraProperties)

	// Updates `CHE_INFRA_KUBERNETES_ADVANCED__AUTHORIZATION__<...>`
	s.updateAdvancedAuthorizationEnv(ctx, cheEnv)

	// Updates `CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES`
	s.updateUserClusterRoleEnv(ctx, cheEnv)

	// Update `CHE_INTEGRATION_<...>_SERVER__ENDPOINTS`
	if err := s.updateServerEndpointsEnv(ctx, cheEnv); err != nil {
		return nil, err
	}

	return cheEnv, nil
}

func (s *CheServerReconciler) updateAdvancedAuthorizationEnv(ctx *chetypes.DeployContext, cheEnv map[string]string) {
	if ctx.CheCluster.Spec.Networking.Auth.AdvancedAuthorization != nil {
		cheEnv["CHE_INFRA_KUBERNETES_ADVANCED__AUTHORIZATION_ALLOW__USERS"] = strings.Join(
			ctx.CheCluster.Spec.Networking.Auth.AdvancedAuthorization.AllowUsers,
			",",
		)
		cheEnv["CHE_INFRA_KUBERNETES_ADVANCED__AUTHORIZATION_DENY__USERS"] = strings.Join(
			ctx.CheCluster.Spec.Networking.Auth.AdvancedAuthorization.DenyUsers,
			",",
		)
		cheEnv["CHE_INFRA_KUBERNETES_ADVANCED__AUTHORIZATION_ALLOW__GROUPS"] = strings.Join(
			ctx.CheCluster.Spec.Networking.Auth.AdvancedAuthorization.AllowGroups,
			",",
		)
		cheEnv["CHE_INFRA_KUBERNETES_ADVANCED__AUTHORIZATION_DENY__GROUPS"] = strings.Join(
			ctx.CheCluster.Spec.Networking.Auth.AdvancedAuthorization.DenyGroups,
			",",
		)
	}
}

func (s *CheServerReconciler) updateUserClusterRoleEnv(ctx *chetypes.DeployContext, cheEnv map[string]string) {
	userClusterRolesSet := map[string]bool{}

	for _, role := range s.getDefaultUserClusterRoles(ctx) {
		userClusterRolesSet[role] = true
	}

	for _, role := range strings.Split(cheEnv["CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"], ",") {
		role = strings.TrimSpace(role)
		if role != "" {
			userClusterRolesSet[strings.TrimSpace(role)] = true
		}
	}

	if ctx.CheCluster.Spec.DevEnvironments.User != nil {
		for _, role := range ctx.CheCluster.Spec.DevEnvironments.User.ClusterRoles {
			role = strings.TrimSpace(role)
			if role != "" {
				userClusterRolesSet[strings.TrimSpace(role)] = true
			}
		}
	}

	userClusterRoles := slices.Collect(maps.Keys(userClusterRolesSet))
	sort.Strings(userClusterRoles)

	cheEnv["CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"] = strings.Join(userClusterRoles, ",")
}

func (s *CheServerReconciler) updateServerEndpointsEnv(ctx *chetypes.DeployContext, cheEnv map[string]string) error {
	// https://github.com/eclipse-che/che-operator/pull/1250
	oAuthProviders := []string{constants.BitbucketOAuth, constants.AzureDevOpsOAuth}

	for _, oauthProvider := range oAuthProviders {
		secret, err := getOAuthConfigSecret(ctx, oauthProvider)
		if err != nil {
			return err
		} else if secret == nil {
			continue
		}

		envName := fmt.Sprintf(
			"CHE_INTEGRATION_%s_SERVER__ENDPOINTS",
			strings.ReplaceAll(
				strings.ToUpper(oauthProvider),
				"-",
				"_",
			),
		)

		// make endpoints uniq and sorted
		endpointsSet := map[string]bool{}

		for _, endpointsStr := range []string{
			secret.Annotations[constants.CheEclipseOrgScmServerEndpoint],
			cheEnv[envName],
		} {
			for _, endpoint := range strings.Split(endpointsStr, ",") {
				endpoint = strings.TrimSpace(endpoint)
				if endpoint != "" {
					endpointsSet[strings.TrimSpace(endpoint)] = true
				}
			}
		}

		endpoints := slices.Collect(maps.Keys(endpointsSet))
		sort.Strings(endpoints)

		cheEnv[envName] = strings.Join(endpoints, ",")
	}

	return nil
}
