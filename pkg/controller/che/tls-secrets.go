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

	"github.com/eclipse/che-operator/pkg/util"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CheTLSJobServiceAccountName           = "che-tls-job-service-account"
	CheTLSJobRoleName                     = "che-tls-job-role"
	CheTLSJobRoleBindingName              = "che-tls-job-role-binding"
	CheTLSJobName                         = "che-tls-job"
	CheTlsJobComponentName                = "che-create-tls-secret-job"
	CheTLSSelfSignedCertificateSecretName = "self-signed-certificate"
)

// HandleCheTLSSecrets handles TLS secrets required for Che deployment
func HandleCheTLSSecrets(checluster *orgv1.CheCluster, clusterAPI deploy.ClusterAPI) (reconcile.Result, error) {
	cheTLSSecretName := checluster.Spec.K8s.TlsSecretName

	// ===== Check Che TLS certificate ===== //
	cheTLSSecret := &corev1.Secret{}
	err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSecretName}, cheTLSSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading secret info
			logrus.Errorf("Error getting Che TLS secert \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		// Che TLS secret doesn't exist, generate a new one

		// Remove Che CA certificate secret if any
		cheCASelfSignedCertificateSecret := &corev1.Secret{}
		err = clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: CheTLSSelfSignedCertificateSecretName}, cheCASelfSignedCertificateSecret)
		if err != nil {
			if !errors.IsNotFound(err) {
				// Error reading secret info
				logrus.Errorf("Error getting Che self-signed certificate secert \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
			// Che CA certificate doesn't exists (that's expected at this point), do nothing
		} else {
			// Remove Che CA secret because Che TLS secret is missing (they should be generated together).
			if err = clusterAPI.Client.Delete(context.TODO(), cheCASelfSignedCertificateSecret); err != nil {
				logrus.Errorf("Error deleting Che self-signed certificate secret \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
		}

		// Prepare permissions for the certificate generation job
		sa, err := deploy.SyncServiceAccountToCluster(checluster, CheTLSJobServiceAccountName, clusterAPI)
		if sa == nil {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		role, err := deploy.SyncRoleToCluster(checluster, CheTLSJobRoleName, []string{"secrets"}, []string{"create"}, clusterAPI)
		if role == nil {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		roleBiding, err := deploy.SyncRoleBindingToCluster(checluster, CheTLSJobRoleBindingName, CheTLSJobServiceAccountName, CheTLSJobRoleName, "Role", clusterAPI)
		if roleBiding == nil {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		cheTLSSecretsCreationJobImage := deploy.DefaultCheTLSSecretsCreationJobImage()
		jobEnvVars := map[string]string{
			"DOMAIN":                         checluster.Spec.K8s.IngressDomain,
			"CHE_NAMESPACE":                  checluster.Namespace,
			"CHE_SERVER_TLS_SECRET_NAME":     cheTLSSecretName,
			"CHE_CA_CERTIFICATE_SECRET_NAME": CheTLSSelfSignedCertificateSecretName,
		}
		job, err := deploy.SyncJobToCluster(checluster, CheTLSJobName, CheTlsJobComponentName, cheTLSSecretsCreationJobImage, CheTLSJobServiceAccountName, jobEnvVars, clusterAPI)
		if err != nil {
			logrus.Error(err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		if job == nil || job.Status.Succeeded == 0 {
			logrus.Infof("Waiting on job '%s' to be finished", CheTLSJobName)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// cleanup job
	job := &batchv1.Job{}
	err = clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheTLSJobName, Namespace: checluster.Namespace}, job)
	if err != nil && !errors.IsNotFound(err) {
		// Failed to get the job
		return reconcile.Result{RequeueAfter: time.Second}, err
	}
	if err == nil {
		// The job object is present
		if job.Status.Succeeded > 0 {
			logrus.Infof("Import public part of Eclipse Che self-signed CA certificvate from \"%s\" secret into your browser.", CheTLSSelfSignedCertificateSecretName)
			deleteJob(job, checluster, clusterAPI)
		} else if job.Status.Failed > 0 {
			// The job failed, but the certificate is present, shouldn't happen
			deleteJob(job, checluster, clusterAPI)
			return reconcile.Result{}, nil
		}
		// Job hasn't reported finished status yet, wait more
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	// Che TLS certificate exists, check for required data fields
	if !isCheTLSSecretValid(cheTLSSecret) {
		// The secret is invalid because required field(s) missing.
		logrus.Infof("Che TLS secret \"%s\" is invalid. Recreating...", cheTLSSecretName)
		// Delete old invalid secret
		if err = clusterAPI.Client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		// Recreate the secret
		return reconcile.Result{RequeueAfter: time.Second}, err
	}

	// Check owner reference
	if cheTLSSecret.ObjectMeta.OwnerReferences == nil {
		// Set owner Che cluster as Che TLS secret owner
		if err := controllerutil.SetControllerReference(checluster, cheTLSSecret, clusterAPI.Scheme); err != nil {
			logrus.Errorf("Failed to set owner for Che TLS secret \"%s\". Error: %s", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		if err := clusterAPI.Client.Update(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Failed to update owner for Che TLS secret \"%s\". Error: %s", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// ===== Check Che CA certificate ===== //

	cheTLSSelfSignedCertificateSecret := &corev1.Secret{}
	err = clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: CheTLSSelfSignedCertificateSecretName}, cheTLSSelfSignedCertificateSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading Che self-signed secret info
			logrus.Errorf("Error getting Che self-signed certificate secert \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		// Che CA self-signed cetificate secret doesn't exist.
		// Such situation could happen between reconcile loops, when CA cert is deleted.
		// However the certificates should be created together,
		// so it is mandatory to remove Che TLS secret now and recreate the pair.
		cheTLSSecret = &corev1.Secret{}
		err = clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSecretName}, cheTLSSecret)
		if err != nil { // No need to check for not found error as the secret already exists at this point
			logrus.Errorf("Error getting Che TLS secert \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		if err = clusterAPI.Client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		// Invalid secrets cleaned up, recreate them now
		return reconcile.Result{RequeueAfter: time.Second}, err
	}

	// Che CA self-signed certificate secret exists, check for required data fields
	if !isCheCASecretValid(cheTLSSelfSignedCertificateSecret) {
		logrus.Infof("Che self-signed certificate secret \"%s\" is invalid. Recrating...", CheTLSSelfSignedCertificateSecretName)
		// Che CA self-signed certificate secret is invalid, delete it
		if err = clusterAPI.Client.Delete(context.TODO(), cheTLSSelfSignedCertificateSecret); err != nil {
			logrus.Errorf("Error deleting Che self-signed certificate secret \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		// Also delete Che TLS as the certificates should be created together
		// Here it is not mandatory to check Che TLS secret existence as it is handled above
		if err = clusterAPI.Client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		// Regenerate Che TLS certicates and recreate secrets
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	// Check owner reference
	if cheTLSSelfSignedCertificateSecret.ObjectMeta.OwnerReferences == nil {
		// Set owner Che cluster as Che TLS secret owner
		if err := controllerutil.SetControllerReference(checluster, cheTLSSelfSignedCertificateSecret, clusterAPI.Scheme); err != nil {
			logrus.Errorf("Failed to set owner for Che self-signed certificate secret \"%s\". Error: %s", CheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		if err := clusterAPI.Client.Update(context.TODO(), cheTLSSelfSignedCertificateSecret); err != nil {
			logrus.Errorf("Failed to update owner for Che self-signed certificate secret \"%s\". Error: %s", CheTLSSelfSignedCertificateSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// Both secrets are ok, go further in reconcile loop
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

// CheckAndUpdateTLSConfiguration validates TLS configuration and precreated objects if any
// If configuration is wrong it will guess most common use cases and will make changes in Che CR accordingly to the assumption.
func CheckAndUpdateTLSConfiguration(checluster *orgv1.CheCluster, clusterAPI deploy.ClusterAPI) error {
	if checluster.Spec.K8s.TlsSecretName == "" {
		checluster.Spec.K8s.TlsSecretName = "che-tls"
		if err := deploy.UpdateCheCRSpec(checluster, "TlsSecretName", checluster.Spec.K8s.TlsSecretName, clusterAPI); err != nil {
			return err
		}
	}

	return nil
}

func deleteJob(job *batchv1.Job, checluster *orgv1.CheCluster, clusterAPI deploy.ClusterAPI) {
	names := util.K8sclient.GetPodsByComponent(CheTlsJobComponentName, checluster.Namespace)
	for _, podName := range names {
		pod := &corev1.Pod{}
		err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: checluster.Namespace}, pod)
		if err == nil {
			// Delete pod (for some reasons pod isn't removed when job is removed)
			if err = clusterAPI.Client.Delete(context.TODO(), pod); err != nil {
				logrus.Errorf("Error deleting pod: '%s', error: %v", podName, err)
			}
		}
	}

	if err := clusterAPI.Client.Delete(context.TODO(), job); err != nil {
		logrus.Errorf("Error deleting job: '%s', error: %v", CheTLSJobName, err)
	}
}
