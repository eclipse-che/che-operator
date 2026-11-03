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

package selfsignedcert

import (
	"fmt"
	"strings"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	cheTLSJobName               = "che-tls-job"
	cheTLSJobComponentName      = "che-create-tls-secret-job"
	cheTLSJobServiceAccountName = "che-tls-job-service-account"
	cheTLSJobRoleName           = "che-tls-job-role"
	cheTLSJobRoleBindingName    = "che-tls-job-role-binding"
)

func (s *SelfSignedCertificateReconciler) syncCheTLSSecrets(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	tlsSecret := &corev1.Secret{}
	tlsSecretExists, err := resources.Secret.Get(
		cheCtx,
		cheCtx.CheCluster.Spec.Networking.TlsSecretName,
		cheCtx.CheCluster.Namespace,
		tlsSecret,
	)
	if err != nil {
		return reconcile.Result{}, false, err
	}
	if !tlsSecretExists {
		return s.triggerTLSJob(cheCtx)
	}

	return reconcile.Result{}, true, nil
}

func (s *SelfSignedCertificateReconciler) triggerTLSJob(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	job := &batchv1.Job{}
	jobExists, err := resources.Job.Get(cheCtx, cheTLSJobName, cheCtx.CheCluster.Namespace, job)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	if jobExists {
		if !resources.Job.IsFinished(job) {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, false, nil
		}
		if resources.Job.IsSucceeded(job) {
			return reconcile.Result{RequeueAfter: time.Second}, false, nil
		}

		// Remove the failed Job first, so that a new one can be started on the
		// next reconcile.
		if err := resources.Job.Delete(cheCtx, cheTLSJobName, cheCtx.CheCluster.Namespace); err != nil {
			return reconcile.Result{}, false, err
		}

		return reconcile.Result{},
			false,
			fmt.Errorf("job %s/%s failed: %s", cheCtx.CheCluster.Namespace, cheTLSJobName, resources.Job.GetFailureReason(job))
	}

	// Remove a leftover self-signed CA certificate secret, if any: the Job generates
	// the Che TLS secret and the CA certificate secret as a matching pair, so a stale
	// CA certificate from a previous run must not outlive the TLS secret being replaced.
	if err := resources.Secret.Delete(cheCtx, constants.SelfSignedCertificateSecretName, cheCtx.CheCluster.Namespace); err != nil {
		return reconcile.Result{}, false, err
	}

	if err := s.syncTLSJob(cheCtx); err != nil {
		return reconcile.Result{}, false, err
	}

	// The Job has just been started and needs time to create the secrets;
	// requeue to poll the Job status until it completes.
	return reconcile.Result{RequeueAfter: 5 * time.Second}, false, nil
}

func (s *SelfSignedCertificateReconciler) syncTLSJobRBAC(cheCtx *chetypes.CheContext) error {
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheTLSJobServiceAccountName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    chelabels.GetStandardLabels(defaults.GetCheFlavor()),
		},
	}
	if err := resources.ServiceAccount.Sync(cheCtx, sa); err != nil {
		return err
	}

	role := &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheTLSJobRoleName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    chelabels.GetStandardLabels(defaults.GetCheFlavor()),
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"secrets",
				},
				Verbs: []string{
					"create",
					"get",
					"patch",
				},
			},
		},
	}
	if err := resources.Role.Sync(cheCtx, role); err != nil {
		return err
	}

	roleBinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheTLSJobRoleBindingName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    chelabels.GetStandardLabels(defaults.GetCheFlavor()),
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      cheTLSJobServiceAccountName,
				Namespace: cheCtx.CheCluster.Namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     cheTLSJobRoleName,
			Kind:     "Role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	if err := resources.RoleBinding.Sync(cheCtx, roleBinding); err != nil {
		return err
	}

	return nil
}

func (s *SelfSignedCertificateReconciler) syncTLSJob(cheCtx *chetypes.CheContext) error {
	if err := s.syncTLSJobRBAC(cheCtx); err != nil {
		return err
	}

	tlsSecretName := cheCtx.CheCluster.Spec.Networking.TlsSecretName

	image := defaults.GetCheTLSSecretsCreationJobImage()
	imagePullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))
	jobLabels := chelabels.GetStandardLabels(cheTLSJobComponentName)

	var secretLabels strings.Builder
	for k, v := range chelabels.GetStandardLabels(tlsSecretName) {
		if _, err := fmt.Fprintf(&secretLabels, "%s=%s ", k, v); err != nil {
			return err
		}
	}

	domain := cheCtx.CheCluster.Spec.Networking.Domain
	domains := domain + ",*." + domain
	if cheCtx.CheHost != "" && !strings.Contains(cheCtx.CheHost, domain) {
		domains += "," + cheCtx.CheHost
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheTLSJobName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    jobLabels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            cheTLSJobServiceAccountName,
					RestartPolicy:                 "Never",
					TerminationGracePeriodSeconds: new(int64(30)),
					DNSPolicy:                     corev1.DNSClusterFirst,
					Containers: []corev1.Container{
						{
							Name:            cheTLSJobName + "-job-container",
							Image:           image,
							ImagePullPolicy: imagePullPolicy,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: new(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								RunAsNonRoot: new(true),
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DOMAIN",
									Value: domains,
								},
								{
									Name:  "CHE_NAMESPACE",
									Value: cheCtx.CheCluster.Namespace,
								},
								{
									Name:  "CHE_SERVER_TLS_SECRET_NAME",
									Value: tlsSecretName,
								},
								{
									Name:  "CHE_CA_CERTIFICATE_SECRET_NAME",
									Value: constants.SelfSignedCertificateSecretName,
								},
								{
									Name:  "LABELS",
									Value: secretLabels.String(),
								},
							},
						},
					},
				},
			},
			TTLSecondsAfterFinished: new(int32(30)),
			Parallelism:             new(int32(1)),
			BackoffLimit:            new(int32(3)),
			Completions:             new(int32(1)),
		},
	}
	if err := resources.Job.Sync(cheCtx, job); err != nil {
		return err
	}

	return nil
}
