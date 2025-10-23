//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
)

var (
	webhookLogger = ctrl.Log.WithName("webhook")
)

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&CheCluster{}).
		WithDefaulter(&CheClusterDefaulter{}).
		WithValidator(&CheClusterValidator{}).
		Complete()
}

// Keep empty line after annotation
// +kubebuilder:webhook:path=/mutate-org-eclipse-che-v2-checluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=org.eclipse.che,resources=checlusters,verbs=create;update,versions=v2,name=mchecluster.kb.io,admissionReviewVersions=v1

type CheClusterDefaulter struct{}

var _ webhook.CustomDefaulter = &CheClusterDefaulter{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CheClusterDefaulter) Default(_ context.Context, obj runtime.Object) error {
	cheCluster, ok := obj.(*CheCluster)

	if !ok {
		return fmt.Errorf("expected an CheCluster object but got %T", obj)
	}

	webhookLogger.Info("Defaulting for CheCluster", "name", cheCluster.GetName())

	r.setDisableContainerRunCapabilities(cheCluster)
	r.setContainerRunConfiguration(cheCluster)

	r.setDisableContainerBuildCapabilities(cheCluster)
	r.setContainerBuildConfiguration(cheCluster)

	return nil
}

func (r *CheClusterDefaulter) setDisableContainerBuildCapabilities(cheCluster *CheCluster) {
	if !infrastructure.IsOpenShift() {
		cheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(true)
	}
}

func (r *CheClusterDefaulter) setDisableContainerRunCapabilities(cheCluster *CheCluster) {
	if !infrastructure.IsOpenShift() {
		cheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = pointer.Bool(true)
	}
}

// Sets ContainerRunConfiguration if container run capabilities is enabled.
// The defaults will be propagated from the CheCluster CRD
func (r *CheClusterDefaulter) setContainerRunConfiguration(cheCluster *CheCluster) {
	if cheCluster.IsContainerRunCapabilitiesEnabled() && cheCluster.Spec.DevEnvironments.ContainerRunConfiguration == nil {
		cheCluster.Spec.DevEnvironments.ContainerRunConfiguration = &ContainerRunConfiguration{}
	}
}

// Sets ContainerBuildConfiguration if container run capabilities is enabled.
// The defaults will be propagated from the CheCluster CRD
func (r *CheClusterDefaulter) setContainerBuildConfiguration(cheCluster *CheCluster) {
	if cheCluster.IsContainerBuildCapabilitiesEnabled() && cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration == nil {
		cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &ContainerBuildConfiguration{}
	}
}

// Keep empty line after annotation
// +kubebuilder:webhook:path=/validate-org-eclipse-che-v2-checluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=org.eclipse.che,resources=checlusters,verbs=create;update,versions=v2,name=vchecluster.kb.io,admissionReviewVersions=v1

type CheClusterValidator struct{}

var _ admission.CustomValidator = &CheClusterValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CheClusterValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	cheCluster, ok := obj.(*CheCluster)

	if !ok {
		return nil, fmt.Errorf("expected an CheCluster object but got %T", obj)
	}

	webhookLogger.Info("Validation for CheCluster upon creation", "name", cheCluster.GetName())

	if err := r.ensureSingletonCheCluster(); err != nil {
		return []string{}, err
	}
	return []string{}, r.validate(cheCluster)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type CheCluster.
func (r *CheClusterValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	cheCluster, ok := newObj.(*CheCluster)

	if !ok {
		return nil, fmt.Errorf("expected an CheCluster object but got %T", newObj)
	}

	webhookLogger.Info("Validation for CheCluster upon update", "name", cheCluster.GetName())

	return nil, r.validate(cheCluster)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type CheCluster.
func (r *CheClusterValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *CheClusterValidator) ensureSingletonCheCluster() error {
	client := k8shelper.New().GetClient()
	utilruntime.Must(AddToScheme(client.Scheme()))

	che := &CheClusterList{}
	err := client.List(context.TODO(), che)
	if err != nil {
		webhookLogger.Error(err, "Failed to list CheCluster Custom Resources.")
	}

	if len(che.Items) != 0 {
		return fmt.Errorf("only one CheCluster is allowed")
	}

	return nil
}

func (r *CheClusterValidator) validate(checluster *CheCluster) error {
	for _, github := range checluster.Spec.GitServices.GitHub {
		if err := r.validateOAuthSecret(github.SecretName, "github", github.Endpoint, github.DisableSubdomainIsolation, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, gitlab := range checluster.Spec.GitServices.GitLab {
		if err := r.validateOAuthSecret(gitlab.SecretName, "gitlab", gitlab.Endpoint, nil, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, bitbucket := range checluster.Spec.GitServices.BitBucket {
		if err := r.validateOAuthSecret(bitbucket.SecretName, "bitbucket", bitbucket.Endpoint, nil, checluster.Namespace); err != nil {
			return err
		}
	}

	for _, azure := range checluster.Spec.GitServices.AzureDevOps {
		if err := r.validateOAuthSecret(azure.SecretName, constants.AzureDevOpsOAuth, "", nil, checluster.Namespace); err != nil {
			return err
		}
	}

	return nil
}

func (r *CheClusterValidator) validateOAuthSecret(secretName string, scmProvider string, serverEndpoint string, disableSubdomainIsolation *bool, namespace string) error {
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

	if err := r.ensureScmLabelsAndAnnotations(secret, scmProvider, serverEndpoint, disableSubdomainIsolation); err != nil {
		return err
	}

	switch scmProvider {
	case "github":
		if err := r.validateGitHubOAuthSecretDataKeys(secret); err != nil {
			return err
		}
	case "gitlab":
		if err := r.validateGitLabOAuthSecretDataKeys(secret); err != nil {
			return err
		}
	case "bitbucket":
		if err := r.validateBitBucketOAuthSecretDataKeys(secret); err != nil {
			return err
		}
	case constants.AzureDevOpsOAuth:
		if err := r.validateAzureDevOpsSecretDataKeys(secret); err != nil {
			return err
		}
	}

	return nil
}

func (r *CheClusterValidator) validateAzureDevOpsSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitHubOAuthConfigClientIdFileName, constants.GitHubOAuthConfigClientSecretFileName}
	return r.validateOAuthSecretDataKeys(secret, keys2validate)
}

func (r *CheClusterValidator) validateGitHubOAuthSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitHubOAuthConfigClientIdFileName, constants.GitHubOAuthConfigClientSecretFileName}
	return r.validateOAuthSecretDataKeys(secret, keys2validate)
}

func (r *CheClusterValidator) validateGitLabOAuthSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitLabOAuthConfigClientIdFileName, constants.GitLabOAuthConfigClientSecretFileName}
	return r.validateOAuthSecretDataKeys(secret, keys2validate)
}

func (r *CheClusterValidator) validateBitBucketOAuthSecretDataKeys(secret *corev1.Secret) error {
	oauth1Keys2validate := []string{constants.BitBucketOAuthConfigPrivateKeyFileName, constants.BitBucketOAuthConfigConsumerKeyFileName}
	errOauth1Keys := r.validateOAuthSecretDataKeys(secret, oauth1Keys2validate)

	oauth2Keys2validate := []string{constants.BitBucketOAuthConfigClientIdFileName, constants.BitBucketOAuthConfigClientSecretFileName}
	errOauth2Keys := r.validateOAuthSecretDataKeys(secret, oauth2Keys2validate)

	if errOauth1Keys != nil && errOauth2Keys != nil {
		return fmt.Errorf("secret must contain either [%s] or [%s] keys", strings.Join(oauth1Keys2validate, ", "), strings.Join(oauth2Keys2validate, ", "))
	}

	return nil
}

func (r *CheClusterValidator) ensureScmLabelsAndAnnotations(secret *corev1.Secret, scmProvider string, serverEndpoint string, disableSubdomainIsolation *bool) error {
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

func (r *CheClusterValidator) validateOAuthSecretDataKeys(secret *corev1.Secret, keys []string) error {
	for _, key := range keys {
		if len(secret.Data[key]) == 0 {
			return fmt.Errorf("secret '%s' must contain [%s] keys", secret.Name, strings.Join(keys, ", "))
		}
	}

	return nil
}
