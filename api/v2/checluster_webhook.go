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

package v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	webhookLogger = ctrl.Log.WithName("webhook")
)

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &CheCluster{}).
		WithDefaulter(&CheClusterDefaulter{}).
		WithValidator(&CheClusterValidator{}).
		Complete()
}

// Keep empty line after annotation
// +kubebuilder:webhook:path=/mutate-org-eclipse-che-v2-checluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=org.eclipse.che,resources=checlusters,verbs=create;update,versions=v2,name=mchecluster.kb.io,admissionReviewVersions=v1

type CheClusterDefaulter struct{}

var _ admission.Defaulter[*CheCluster] = &CheClusterDefaulter{}

// Default implements admission.Defaulter so a webhook will be registered for the type
func (r *CheClusterDefaulter) Default(_ context.Context, cheCluster *CheCluster) error {
	webhookLogger.Info("Defaulting for CheCluster", "name", cheCluster.GetName())

	r.setDisableContainerRunCapabilities(cheCluster)
	r.setContainerRunConfiguration(cheCluster)

	r.setDisableContainerBuildCapabilities(cheCluster)
	r.setContainerBuildConfiguration(cheCluster)

	return nil
}

func (r *CheClusterDefaulter) setDisableContainerBuildCapabilities(cheCluster *CheCluster) {
	if !infrastructure.IsOpenShift() {
		cheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = ptr.To(true)
	}
}

func (r *CheClusterDefaulter) setDisableContainerRunCapabilities(cheCluster *CheCluster) {
	if !infrastructure.IsOpenShift() {
		cheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = ptr.To(true)
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

var _ admission.Validator[*CheCluster] = &CheClusterValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type
func (r *CheClusterValidator) ValidateCreate(ctx context.Context, cheCluster *CheCluster) (admission.Warnings, error) {
	webhookLogger.Info("Validation for CheCluster upon creation", "name", cheCluster.GetName())

	if err := r.ensureSingletonCheCluster(ctx); err != nil {
		return []string{}, err
	}
	return []string{}, r.validate(ctx, cheCluster)
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type CheCluster.
func (r *CheClusterValidator) ValidateUpdate(ctx context.Context, _, newObj *CheCluster) (admission.Warnings, error) {
	webhookLogger.Info("Validation for CheCluster upon update", "name", newObj.GetName())

	return nil, r.validate(ctx, newObj)
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type CheCluster.
func (r *CheClusterValidator) ValidateDelete(_ context.Context, _ *CheCluster) (admission.Warnings, error) {
	return nil, nil
}

func (r *CheClusterValidator) ensureSingletonCheCluster(ctx context.Context) error {
	che := &CheClusterList{}
	err := clusterapi.Get().NonCachingClient.List(ctx, che)
	if err != nil {
		return fmt.Errorf("failed to list CheClusters: %w", err)
	}

	if len(che.Items) != 0 {
		return fmt.Errorf("only one CheCluster is allowed")
	}

	return nil
}

func (r *CheClusterValidator) validate(ctx context.Context, checluster *CheCluster) error {
	if !checluster.DeletionTimestamp.IsZero() {
		return nil
	}

	if err := r.validateOpenVSXRegistry(ctx, checluster); err != nil {
		return err
	}

	if err := r.validateAgentSandbox(checluster); err != nil {
		return err
	}

	if err := r.validateNetworkingDomain(checluster); err != nil {
		return err
	}

	for _, github := range checluster.Spec.GitServices.GitHub {
		if err := r.validateOAuthSecret(
			ctx,
			github.SecretName,
			"github",
			github.Endpoint,
			github.DisableSubdomainIsolation,
			checluster.Namespace,
		); err != nil {
			return err
		}
	}

	for _, gitlab := range checluster.Spec.GitServices.GitLab {
		if err := r.validateOAuthSecret(
			ctx,
			gitlab.SecretName,
			"gitlab",
			gitlab.Endpoint,
			nil,
			checluster.Namespace,
		); err != nil {
			return err
		}
	}

	for _, bitbucket := range checluster.Spec.GitServices.BitBucket {
		if err := r.validateOAuthSecret(
			ctx,
			bitbucket.SecretName,
			"bitbucket",
			bitbucket.Endpoint,
			nil,
			checluster.Namespace,
		); err != nil {
			return err
		}
	}

	for _, azure := range checluster.Spec.GitServices.AzureDevOps {
		if err := r.validateOAuthSecret(
			ctx,
			azure.SecretName,
			constants.AzureDevOpsOAuth,
			"",
			nil,
			checluster.Namespace,
		); err != nil {
			return err
		}
	}

	return nil
}

func (r *CheClusterValidator) validateOAuthSecret(
	ctx context.Context,
	secretName string,
	scmProvider string,
	serverEndpoint string,
	disableSubdomainIsolation *bool,
	namespace string,
) error {
	if secretName == "" {
		return nil
	}

	secret := &corev1.Secret{}
	key := client.ObjectKey{Name: secretName, Namespace: namespace}

	err := clusterapi.Get().NonCachingClient.Get(ctx, key, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("secret '%s' not found", secretName)
		}
		return fmt.Errorf("error reading '%s' secret: %w", secretName, err)
	}

	if err := r.ensureScmLabelsAndAnnotations(
		ctx,
		secret,
		scmProvider,
		serverEndpoint,
		disableSubdomainIsolation,
	); err != nil {
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
	return r.validateSecretDataKeys(secret, keys2validate)
}

func (r *CheClusterValidator) validateGitHubOAuthSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitHubOAuthConfigClientIdFileName, constants.GitHubOAuthConfigClientSecretFileName}
	return r.validateSecretDataKeys(secret, keys2validate)
}

func (r *CheClusterValidator) validateGitLabOAuthSecretDataKeys(secret *corev1.Secret) error {
	keys2validate := []string{constants.GitLabOAuthConfigClientIdFileName, constants.GitLabOAuthConfigClientSecretFileName}
	return r.validateSecretDataKeys(secret, keys2validate)
}

func (r *CheClusterValidator) validateBitBucketOAuthSecretDataKeys(secret *corev1.Secret) error {
	oauth1Keys2validate := []string{constants.BitBucketOAuthConfigPrivateKeyFileName, constants.BitBucketOAuthConfigConsumerKeyFileName}
	errOauth1Keys := r.validateSecretDataKeys(secret, oauth1Keys2validate)

	oauth2Keys2validate := []string{constants.BitBucketOAuthConfigClientIdFileName, constants.BitBucketOAuthConfigClientSecretFileName}
	errOauth2Keys := r.validateSecretDataKeys(secret, oauth2Keys2validate)

	if errOauth1Keys != nil && errOauth2Keys != nil {
		return fmt.Errorf("secret must contain either [%s] or [%s] keys", strings.Join(oauth1Keys2validate, ", "), strings.Join(oauth2Keys2validate, ", "))
	}

	return nil
}

func (r *CheClusterValidator) ensureScmLabelsAndAnnotations(
	ctx context.Context,
	secret *corev1.Secret,
	scmProvider string,
	serverEndpoint string,
	disableSubdomainIsolation *bool,
) error {
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
		// for backward compatibility, copy CheCluster CR value into annotation
		patch.Annotations[constants.CheEclipseOrgScmGitHubDisableSubdomainIsolation] = strconv.FormatBool(*disableSubdomainIsolation)
	}
	if serverEndpoint != "" && secret.Annotations[constants.CheEclipseOrgScmServerEndpoint] == "" {
		// for backward compatibility, copy CheCluster CR value into annotation
		patch.Annotations[constants.CheEclipseOrgScmServerEndpoint] = serverEndpoint
	}

	patchData, _ := json.Marshal(patch)
	rawPatch := client.RawPatch(types.MergePatchType, patchData)

	if err := clusterapi.Get().NonCachingClient.Patch(ctx, secret, rawPatch); err != nil {
		return fmt.Errorf("failed to patch Secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	return nil
}

func (r *CheClusterValidator) validateOpenVSXRegistry(ctx context.Context, checluster *CheCluster) error {
	if checluster.Spec.Components.OpenVSXRegistry.Database != nil &&
		checluster.Spec.Components.OpenVSXRegistry.Database.Storage != nil &&
		checluster.Spec.Components.OpenVSXRegistry.Database.Storage.ClaimSize != "" {

		if _, err := resource.ParseQuantity(checluster.Spec.Components.OpenVSXRegistry.Database.Storage.ClaimSize); err != nil {
			return fmt.Errorf("invalid value for OpenVSX database PVC claim size: %w", err)
		}
	}

	if checluster.Spec.Components.OpenVSXRegistry.Server != nil &&
		checluster.Spec.Components.OpenVSXRegistry.Server.Storage != nil &&
		checluster.Spec.Components.OpenVSXRegistry.Server.Storage.ClaimSize != "" {

		if _, err := resource.ParseQuantity(checluster.Spec.Components.OpenVSXRegistry.Server.Storage.ClaimSize); err != nil {
			return fmt.Errorf("invalid value for OpenVSX server PVC claim size: %w", err)
		}
	}

	if checluster.Spec.Components.OpenVSXRegistry.CredentialsSecretName != nil {
		credentialsSecretName := *checluster.Spec.Components.OpenVSXRegistry.CredentialsSecretName

		secret := &corev1.Secret{}
		key := client.ObjectKey{Name: credentialsSecretName, Namespace: checluster.Namespace}

		err := clusterapi.Get().NonCachingClient.Get(ctx, key, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("OpenVSX registry credentials secret %s not found", credentialsSecretName)
			}

			return fmt.Errorf("failed to get OpenVSX registry credentials secret: %w", err)
		}

		if err := r.validateSecretDataKeys(secret,
			[]string{
				"database-user",
				"database-password",
				"database-name",
				"openvsx-publisher-name",
				"openvsx-publisher-token",
				"openvsx-admin-name",
				"openvsx-admin-token",
			}); err != nil {
			return err
		}
	}

	return nil
}

func (r *CheClusterValidator) validateSecretDataKeys(secret *corev1.Secret, keys []string) error {
	for _, key := range keys {
		if value, ok := secret.Data[key]; !ok || len(value) == 0 {
			return fmt.Errorf("mandatory keys [%s] not found in secret %s", strings.Join(keys, ", "), secret.Name)
		}
	}

	return nil
}

func (r *CheClusterValidator) validateNetworkingDomain(cheCluster *CheCluster) error {
	if !infrastructure.IsOpenShift() {
		if cheCluster.Spec.Networking.Domain == "" {
			return fmt.Errorf("required field `spec.networking.domain` is not set; set it to the base domain of the cluster ingress")
		}
	}

	return nil
}

func (r *CheClusterValidator) validateAgentSandbox(cheCluster *CheCluster) error {
	if cheCluster.IsAgentSandboxEnabled() &&
		!infrastructure.IsAgentSandboxEnabled(clusterapi.Get().DiscoveryClient) {

		return fmt.Errorf("AgentSandbox integration cannot be enabled because the agents.x-k8s.io API group is not available in the cluster")
	}

	return nil
}
