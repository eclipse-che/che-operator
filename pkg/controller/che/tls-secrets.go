//
// Copyright (c) 2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package che

import (
	"context"
	"time"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// shouldReturn specifies whether caller of HandleCheTLSSecrets should return from reconcile loop.
var shouldReturn = true

// HandleCheTLSSecrets handles TLS secrets required for Che deployment
func HandleCheTLSSecrets(checluster *orgv1.CheCluster, r *ReconcileChe) (bool, reconcile.Result, error) {
	shouldReturn = true
	reconcileResult, err := processCheTLSSecrets(checluster, r)
	return shouldReturn, reconcileResult, err
}

func processCheTLSSecrets(checluster *orgv1.CheCluster, r *ReconcileChe) (reconcile.Result, error) {
	var cheTLSSecretName = checluster.Spec.K8s.TlsSecretName
	const cheTLSSelfSignedCertificateSecretName string = "self-signed-certificate"

	// ===== Check Che TLS certificate ===== //

	cheTLSSecret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSecretName}, cheTLSSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading secret info
			logrus.Errorf("Error getting Che secert \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}

		// Che TLS secret doesn't exist, generate a new one

		// Remove Che CA certificate secret if any
		cheCASelfSignedCertificateSecret := &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSelfSignedCertificateSecretName}, cheCASelfSignedCertificateSecret)
		if err != nil {
			if !errors.IsNotFound(err) {
				// Error reading secret info
				logrus.Errorf("Error getting Che secert \"%s\": %v", cheTLSSecretName, err)
				return reconcile.Result{}, err
			}
			// Che CA certificate doesn't exists (that's expected at this point), do nothing
		} else {
			// Remove Che CA secret because Che TLS secret is missing (they should be generated together).
			if err = r.client.Delete(context.TODO(), cheCASelfSignedCertificateSecret); err != nil {
				logrus.Errorf("Error deleting Che TLS secret: %v", err)
				return reconcile.Result{}, err
			}
		}

		// Prepare permissions for the certificate generation job

		const cheTLSJobServiceAccountName string = "che-tls-job-service-account"
		cheTLSJobServiceAccount := &corev1.ServiceAccount{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSJobServiceAccountName}, cheTLSJobServiceAccount)
		if err != nil {
			if !errors.IsNotFound(err) {
				logrus.Errorf("Error getting Che TLS job service account \"%s\": %v", cheTLSJobServiceAccountName, err)
				return reconcile.Result{}, err
			}

			// Che TLS job service accound doesn't exist, creating one
			cheTLSJobServiceAccount = deploy.NewServiceAccount(checluster, cheTLSJobServiceAccountName)
			if err := r.CreateServiceAccount(checluster, cheTLSJobServiceAccount); err != nil {
				logrus.Errorf("Error creating Che TLS job service account \"%s\": %v", cheTLSJobServiceAccountName, err)
				return reconcile.Result{}, err
			}
		}

		const cheTLSJobRoleName string = "che-tls-job-role"
		cheTLSJobRole := &rbac.Role{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSJobRoleName}, cheTLSJobRole)
		if err != nil {
			if !errors.IsNotFound(err) {
				logrus.Errorf("Error getting Che TLS job role \"%s\": %v", cheTLSJobRoleName, err)
				return reconcile.Result{}, err
			}

			// Che TLS job role doesn't exist, creating one
			cheTLSJobRole = deploy.NewRole(checluster, cheTLSJobRoleName, []string{"secrets"}, []string{"create"})
			if err := r.CreateNewRole(checluster, cheTLSJobRole); err != nil {
				logrus.Errorf("Error creating Che TLS job role \"%s\": %v", cheTLSJobRoleName, err)
				return reconcile.Result{}, err
			}
		}

		const cheTLSJobRoleBindingName string = "che-tls-job-role-binding"
		cheTLSJobRoleBinding := &rbac.RoleBinding{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSJobRoleBindingName}, cheTLSJobRoleBinding)
		if err != nil {
			if !errors.IsNotFound(err) {
				logrus.Errorf("Error getting Che TLS job role binding \"%s\": %v", cheTLSJobRoleBindingName, err)
				return reconcile.Result{}, err
			}

			// Che TLS job role binding doesn't exist, creating one
			cheTLSJobRoleBinding = deploy.NewRoleBinding(checluster, cheTLSJobRoleBindingName, cheTLSJobServiceAccountName, cheTLSJobRoleName, "Role")
			if err = r.CreateNewRoleBinding(checluster, cheTLSJobRoleBinding); err != nil {
				logrus.Errorf("Error creating Che TLS job role binding \"%s\": %v", cheTLSJobRoleBindingName, err)
				return reconcile.Result{}, err
			}
		}

		// Check Che TLS job existence
		const cheTLSJobName string = "che-tls-job"
		cheTLSJob := &batchv1.Job{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSJobName}, cheTLSJob)
		if err != nil && !errors.IsNotFound(err) {
			logrus.Errorf("Error gitting Che TLS job \"%s\": %v", cheTLSJobRoleBindingName, err)
			return reconcile.Result{}, err
		}
		if err == nil {
			// Che TLS job has been created before

			// Check the job status
			if !(cheTLSJob.Status.Succeeded > 0 || cheTLSJob.Status.Failed > 0) {
				// The job hasn't completed yet, wait more
				return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
			}

			// Che TLS job is finished, clean it up
			if err = r.client.Delete(context.TODO(), cheTLSJob); err != nil {
				logrus.Errorf("Error deleting Che TLS job \"%s\": %v", cheTLSJobName, err)
				return reconcile.Result{}, err
			}

			// Continue in the next reconcile loop if job succeeded
			if cheTLSJob.Status.Succeeded > 0 {
				// Requeue because despite the job is suceeded, it is required to check the resulting secrets
				return reconcile.Result{Requeue: true}, nil
			}

			// Create the job again if it failed
		}

		// Create and run Che TLS job
		// Prepare job configuration
		cheTLSSecretsCreationJobImage := deploy.DefaultCheTLSSecretsCreationJobImage()
		jobEnvVars := map[string]string{
			"DOMAIN":                         checluster.Spec.K8s.IngressDomain,
			"CHE_NAMESPACE":                  checluster.Namespace,
			"CHE_SERVER_TLS_SECRET_NAME":     cheTLSSecretName,
			"CHE_CA_CERTIFICATE_SECRET_NAME": cheTLSSelfSignedCertificateSecretName,
		}
		// Create and run the job
		cheTLSJob = deploy.NewJob(checluster, cheTLSJobName, checluster.Namespace, cheTLSSecretsCreationJobImage, cheTLSJobServiceAccountName, jobEnvVars, 1)
		err = r.CreateNewJob(checluster, cheTLSJob)
		if err != nil {
			logrus.Errorf("Error creating Che TLS job \"%s\": %v", cheTLSJobName, err)
			return reconcile.Result{}, err
		}

		// Give some time for the job to complete.
		// It is not critical if the job will not be completed till next reconcile loop, such situation is handled above.
		return reconcile.Result{Requeue: true}, nil
	}

	// Che TLS certificate exists, check for required data fields
	if !isCheTLSSecretValid(cheTLSSecret) {
		// The secret is invalid because required field(s) missing.
		logrus.Infof("Che TLS secret \"%s\" is invalid. Recrating...", cheTLSSecretName)
		// Delete old invalid secret
		if err = r.client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret: %v", err)
			return reconcile.Result{}, err
		}
		// Recreate the secret
		return reconcile.Result{Requeue: true}, nil
	}

	// Check owner reference
	if cheTLSSecret.ObjectMeta.OwnerReferences == nil {
		// Set owner Che cluster as Che TLS secret owner
		if err := controllerutil.SetControllerReference(checluster, cheTLSSecret, r.scheme); err != nil {
			logrus.Errorf("Failed to set owner for \"%s\" secret. Error: %s", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}
		if err := r.client.Update(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Failed to update owner for \"%s\" secret. Error: %s", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}
	}

	// ===== Check Che CA certificate ===== //

	cheTLSSelfSignedCertificateSecret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSelfSignedCertificateSecretName}, cheTLSSelfSignedCertificateSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading Che self-signed secret info
			logrus.Errorf("Error getting Che self-signedsecert \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}
		// Che CA self-signed cetificate secret doesn't exist.
		// Such situation could happen between reconcile loops, when CA cert is deleted.
		// However the certificates should be created together,
		// so it is mandatory to remove Che TLS secret now and recreate the pair.
		cheTLSSecret = &corev1.Secret{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSecretName}, cheTLSSecret)
		if err != nil { // No need to check for not found error as the secret already exists at this point
			logrus.Errorf("Error getting Che secert \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}
		if err = r.client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}
		// Invalid secrets cleaned up, recreate them now
		return reconcile.Result{Requeue: true}, nil
	}

	// Che CA self-signed certificate secret exists, check for required data fields
	if !isCheCASecretValid(cheTLSSelfSignedCertificateSecret) {
		logrus.Infof("Che self signed certificate secret \"%s\" is invalid. Recrating...", cheTLSSelfSignedCertificateSecretName)
		// Che CA self-signed certificate secret is invalid, delete it
		if err = r.client.Delete(context.TODO(), cheTLSSelfSignedCertificateSecret); err != nil {
			logrus.Errorf("Error deleting Che self-signed secret \"%s\": %v", cheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{}, err
		}
		// Also delete Che TLS as the certificates should be created together
		// Here it is not mandatory to check Che TLS secret existence as it is handled above
		if err = r.client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{}, err
		}
		// Regenerate Che TLS certicates and recreate secrets
		return reconcile.Result{Requeue: true}, nil
	}

	// Check owner reference
	if cheTLSSelfSignedCertificateSecret.ObjectMeta.OwnerReferences == nil {
		// Set owner Che cluster as Che TLS secret owner
		if err := controllerutil.SetControllerReference(checluster, cheTLSSelfSignedCertificateSecret, r.scheme); err != nil {
			logrus.Errorf("Failed to set owner for \"%s\" secret. Error: %s", cheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{}, err
		}
		if err := r.client.Update(context.TODO(), cheTLSSelfSignedCertificateSecret); err != nil {
			logrus.Errorf("Failed to update owner for \"%s\" secret. Error: %s", cheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{}, err
		}
	}

	// Both secrets are ok, go further in reconcile loop
	shouldReturn = false
	return reconcile.Result{}, nil
}

func isCheTLSSecretValid(cheTLSSecret *corev1.Secret) bool {
	if data, exists := cheTLSSecret.Data["tls.key"]; !exists || len(data) == 0 {
		return false
	}
	if data, exists := cheTLSSecret.Data["tls.crt"]; !exists || len(data) == 0 {
		return false
	}
	return true
}

func isCheCASecretValid(cheCASelfSignedCertificateSecret *corev1.Secret) bool {
	if data, exists := cheCASelfSignedCertificateSecret.Data["ca.crt"]; !exists || len(data) == 0 {
		return false
	}
	return true
}
