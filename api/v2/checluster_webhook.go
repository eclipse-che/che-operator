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

package v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
	"k8s.io/utils/pointer"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	logger = ctrl.Log.WithName("webhook")
)

func (r *CheCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &CheCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CheCluster) Default() {
	setContainerBuildConfiguration(r)
	setDefaultOpenVSXURL(r)
	setDefaultMaxNumberOfRunningWorkspacesPerUser(r)
}

// Sets ContainerBuildConfiguration if container build capabilities is enabled.
func setContainerBuildConfiguration(cheCluster *CheCluster) {
	if cheCluster.IsContainerBuildCapabilitiesEnabled() && cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration == nil {
		cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &ContainerBuildConfiguration{}
	}
}

// https://github.com/eclipse/che/issues/21637
// When installing Che, the default CheCluster should have pluginRegistry.openVSXURL set to https://open-vsx.org.
// When updating Che v7.52 or earlier, if `openVSXURL` is NOT set then we should set it to https://open-vsx.org.
// When updating Che v7.53 or later, if `openVSXURL` is NOT set then we should not modify it.
func setDefaultOpenVSXURL(cheCluster *CheCluster) {
	if cheCluster.IsAirGapMode() {
		// don't set any default value, since it causes the workspace to fail to start.
		return
	}

	if cheCluster.Spec.Components.PluginRegistry.OpenVSXURL == nil {
		if cheCluster.Status.CheVersion == "" {
			// Eclipse Che is being installed, then set default
			cheCluster.Spec.Components.PluginRegistry.OpenVSXURL = pointer.StringPtr(constants.DefaultOpenVSXUrl)
			return
		}

		if cheCluster.IsCheFlavor() &&
			cheCluster.Status.CheVersion != "" &&
			cheCluster.Status.CheVersion != "next" &&
			semver.Compare(fmt.Sprintf("v%s", cheCluster.Status.CheVersion), "v7.53.0") == -1 {
			// Eclipse Che is being updated from version < 7.53.0
			cheCluster.Spec.Components.PluginRegistry.OpenVSXURL = pointer.StringPtr(constants.DefaultOpenVSXUrl)
			return
		}
	}
}

// setDefaultMaxNumberOfRunningWorkspacesPerUser copies from `RunningLimit` field into `MaxNumberOfRunningWorkspacesPerUser` since
// Spec.Components.DevWorkspace.RunningLimit is deprecated in favor of Spec.DevEnvironments.MaxNumberOfRunningWorkspacesPerUser
func setDefaultMaxNumberOfRunningWorkspacesPerUser(cheCluster *CheCluster) {
	if cheCluster.Spec.Components.DevWorkspace.RunningLimit != "" {
		if runningLimit, err := strconv.ParseInt(cheCluster.Spec.Components.DevWorkspace.RunningLimit, 10, 64); err == nil {
			cheCluster.Spec.Components.DevWorkspace.RunningLimit = ""
			cheCluster.Spec.DevEnvironments.MaxNumberOfRunningWorkspacesPerUser = pointer.Int64Ptr(runningLimit)
		} else {
			logger.Error(err, "Failed to parse", "spec.components.devWorkspace.runningLimit", cheCluster.Spec.Components.DevWorkspace.RunningLimit)
		}
	}
}

var _ webhook.Validator = &CheCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CheCluster) ValidateCreate() error {
	if err := ensureSingletonCheCluster(); err != nil {
		return err
	}
	return validate(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CheCluster) ValidateUpdate(old runtime.Object) error {
	return validate(r)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CheCluster) ValidateDelete() error {
	return nil
}

func ensureSingletonCheCluster() error {
	client := k8shelper.New().GetClient()
	utilruntime.Must(AddToScheme(client.Scheme()))

	che := &CheClusterList{}
	err := client.List(context.TODO(), che)
	if err != nil {
		logger.Error(err, "Failed to list CheCluster Custom Resources.")
	}

	if len(che.Items) != 0 {
		return fmt.Errorf("only one CheCluster is allowed")
	}

	return nil
}

func validate(checluster *CheCluster) error {
	for _, github := range checluster.Spec.GitServices.GitHub {
		if err := validateGitHubOAuthSecret(github, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, gitlab := range checluster.Spec.GitServices.GitLab {
		if err := validateGitLabOAuthSecret(gitlab, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, bitbucket := range checluster.Spec.GitServices.BitBucket {
		if err := validateBitBucketOAuthSecret(bitbucket, checluster.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func validateGitHubOAuthSecret(github GitHubService, namespace string) error {
	if github.SecretName == "" {
		return nil
	}

	keys2validate := []string{constants.GitHubOAuthConfigClientIdFileName, constants.GitHubOAuthConfigClientSecretFileName}
	if err := validateSecretKeys(keys2validate, github.SecretName, namespace); err != nil {
		return err
	}
	if err := ensureScmLabelsAndAnnotations("github", github.Endpoint, github.SecretName, namespace); err != nil {
		return err
	}
	return nil
}

func validateGitLabOAuthSecret(gitlab GitLabService, namespace string) error {
	if gitlab.SecretName == "" {
		return nil
	}

	keys2validate := []string{constants.GitLabOAuthConfigClientIdFileName, constants.GitLabOAuthConfigClientSecretFileName}
	if err := validateSecretKeys(keys2validate, gitlab.SecretName, namespace); err != nil {
		return err
	}
	if err := ensureScmLabelsAndAnnotations("gitlab", gitlab.Endpoint, gitlab.SecretName, namespace); err != nil {
		return err
	}
	return nil
}

func validateBitBucketOAuthSecret(bitbucket BitBucketService, namespace string) error {
	if bitbucket.SecretName == "" {
		return nil
	}

	oauth1Keys2validate := []string{constants.BitBucketOAuthConfigPrivateKeyFileName, constants.BitBucketOAuthConfigConsumerKeyFileName}
	errOauth1Keys := validateSecretKeys(oauth1Keys2validate, bitbucket.SecretName, namespace)
	oauth2Keys2validate := []string{constants.BitBucketOAuthConfigClientIdFileName, constants.BitBucketOAuthConfigClientSecretFileName}
	errOauth2Keys := validateSecretKeys(oauth2Keys2validate, bitbucket.SecretName, namespace)
	if errOauth1Keys != nil && errOauth2Keys != nil {
		return fmt.Errorf("secret must contain either [%s] or [%s] keys", strings.Join(oauth1Keys2validate, ", "), strings.Join(oauth2Keys2validate, ", "))
	}
	if err := ensureScmLabelsAndAnnotations("bitbucket", bitbucket.Endpoint, bitbucket.SecretName, namespace); err != nil {
		return err
	}
	return nil
}

func ensureScmLabelsAndAnnotations(scmProvider string, endpointUrl string, secretName string, namespace string) error {
	patch := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.CheEclipseOrgOAuthScmServer:    scmProvider,
				constants.CheEclipseOrgScmServerEndpoint: endpointUrl,
			},
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: constants.OAuthScmConfiguration,
			},
		},
	}
	patchData, _ := json.Marshal(patch)

	k8sHelper := k8shelper.New()
	if _, err := k8sHelper.GetClientset().CoreV1().Secrets(namespace).Patch(context.TODO(), secretName, types.MergePatchType, patchData, metav1.PatchOptions{}); err != nil {
		return err
	}

	return nil
}

func validateSecretKeys(keys []string, secretName string, namespace string) error {
	k8sHelper := k8shelper.New()
	secret, err := k8sHelper.GetClientset().CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for _, key := range keys {
		if len(secret.Data[key]) == 0 {
			return fmt.Errorf("secret must contain [%s] keys", strings.Join(keys, ", "))
		}
	}

	return nil
}
