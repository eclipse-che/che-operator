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

package v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"k8s.io/utils/pointer"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (r *CheCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &CheCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CheCluster) Default() {
	setContainerBuildConfiguration(r)
	setDisableContainerBuildCapabilities(r)
}

func setDisableContainerBuildCapabilities(cheCluster *CheCluster) {
	// Container build capabilities can be enabled on OpenShift only
	if !infrastructure.IsOpenShift() {
		cheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(true)
	}
}

// Sets ContainerBuildConfiguration if container build capabilities is enabled.
func setContainerBuildConfiguration(cheCluster *CheCluster) {
	if cheCluster.IsContainerBuildCapabilitiesEnabled() && cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration == nil {
		cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &ContainerBuildConfiguration{}
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
		if err := validateOAuthSecret(github.SecretName, "github", github.Endpoint, github.DisableSubdomainIsolation, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, gitlab := range checluster.Spec.GitServices.GitLab {
		if err := validateOAuthSecret(gitlab.SecretName, "gitlab", gitlab.Endpoint, nil, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, bitbucket := range checluster.Spec.GitServices.BitBucket {
		if err := validateOAuthSecret(bitbucket.SecretName, "bitbucket", bitbucket.Endpoint, nil, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, azure := range checluster.Spec.GitServices.AzureDevOps {
		if err := validateOAuthSecret(azure.SecretName, constants.AzureDevOpsOAuth, "", nil, checluster.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func validateOAuthSecret(secretName string, scmProvider string, serverEndpoint string, disableSubdomainIsolation *bool, namespace string) error {
	if secretName == "" {
		return nil
	}

	k8sHelper := k8shelper.New()
	secret, err := k8sHelper.GetClientset().CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("secret '%s' not found", secretName)
		}
		return fmt.Errorf("error reading '%s' secret", err.Error())
	}

	if err := ensureScmLabelsAndAnnotations(secret, scmProvider, serverEndpoint, disableSubdomainIsolation); err != nil {
		return err
	}

	switch scmProvider {
	case "github":
		if err := validateGitHubOAuthSecretDataKeys(secret); err != nil {
			return err
		}
	case "gitlab":
		if err := validateGitLabOAuthSecretDataKeys(secret); err != nil {
			return err
		}
	case "bitbucket":
		if err := validateBitBucketOAuthSecretDataKeys(secret); err != nil {
			return err
		}
	case constants.AzureDevOpsOAuth:
		if err := validateAzureDevOpsSecretDataKeys(secret); err != nil {
			return err
		}
	}

	return nil
}

func validateAzureDevOpsSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitHubOAuthConfigClientIdFileName, constants.GitHubOAuthConfigClientSecretFileName}
	return validateOAuthSecretDataKeys(secret, keys2validate)
}

func validateGitHubOAuthSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitHubOAuthConfigClientIdFileName, constants.GitHubOAuthConfigClientSecretFileName}
	return validateOAuthSecretDataKeys(secret, keys2validate)
}

func validateGitLabOAuthSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitLabOAuthConfigClientIdFileName, constants.GitLabOAuthConfigClientSecretFileName}
	return validateOAuthSecretDataKeys(secret, keys2validate)
}

func validateBitBucketOAuthSecretDataKeys(secret *corev1.Secret) error {
	oauth1Keys2validate := []string{constants.BitBucketOAuthConfigPrivateKeyFileName, constants.BitBucketOAuthConfigConsumerKeyFileName}
	errOauth1Keys := validateOAuthSecretDataKeys(secret, oauth1Keys2validate)

	oauth2Keys2validate := []string{constants.BitBucketOAuthConfigClientIdFileName, constants.BitBucketOAuthConfigClientSecretFileName}
	errOauth2Keys := validateOAuthSecretDataKeys(secret, oauth2Keys2validate)

	if errOauth1Keys != nil && errOauth2Keys != nil {
		return fmt.Errorf("secret must contain either [%s] or [%s] keys", strings.Join(oauth1Keys2validate, ", "), strings.Join(oauth2Keys2validate, ", "))
	}

	return nil
}

func ensureScmLabelsAndAnnotations(secret *corev1.Secret, scmProvider string, serverEndpoint string, disableSubdomainIsolation *bool) error {
	patch := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.CheEclipseOrgOAuthScmServer: scmProvider,
			},
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: constants.OAuthScmConfiguration,
			},
		},
	}

	if disableSubdomainIsolation != nil && secret.Annotations[constants.CheEclipseOrgScmGitHubDisableSubdomainIsolation] == "" {
		// for backward compatability, copy CheCluster CR value into annotation
		patch.Annotations[constants.CheEclipseOrgScmGitHubDisableSubdomainIsolation] = strconv.FormatBool(*disableSubdomainIsolation)
	}
	if serverEndpoint != "" && secret.Annotations[constants.CheEclipseOrgScmServerEndpoint] == "" {
		// for backward compatability, copy CheCluster CR value into annotation
		patch.Annotations[constants.CheEclipseOrgScmServerEndpoint] = serverEndpoint
	}

	patchData, _ := json.Marshal(patch)
	k8sHelper := k8shelper.New()
	if _, err := k8sHelper.
		GetClientset().
		CoreV1().
		Secrets(secret.Namespace).
		Patch(context.TODO(), secret.Name, types.MergePatchType, patchData, metav1.PatchOptions{}); err != nil {
		return err
	}

	return nil
}

func validateOAuthSecretDataKeys(secret *corev1.Secret, keys []string) error {
	for _, key := range keys {
		if len(secret.Data[key]) == 0 {
			return fmt.Errorf("secret '%s' must contain [%s] keys", secret.Name, strings.Join(keys, ", "))
		}
	}

	return nil
}
