//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	stderrors "errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TLS related constants
const (
	CheTLSJobServiceAccountName           = "che-tls-job-service-account"
	CheTLSJobRoleName                     = "che-tls-job-role"
	CheTLSJobRoleBindingName              = "che-tls-job-role-binding"
	CheTLSJobName                         = "che-tls-job"
	CheTLSJobComponentName                = "che-create-tls-secret-job"
	CheTLSSelfSignedCertificateSecretName = "self-signed-certificate"
	DefaultCheTLSSecretName               = "che-tls"
)

// IsSelfSignedCertificateUsed detects whether endpoints are/should be secured by self-signed certificate.
func IsSelfSignedCertificateUsed(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) (bool, error) {
	if util.IsTestMode() {
		return true, nil
	}

	cheTLSSelfSignedCertificateSecret := &corev1.Secret{}
	err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: CheTLSSelfSignedCertificateSecretName}, cheTLSSelfSignedCertificateSecret)
	if err == nil {
		// "self signed-certificate" secret found
		return true, nil
	}
	if !errors.IsNotFound(err) {
		// Failed to get secret, return error to restart reconcile loop.
		return false, err
	}

	if util.IsOpenShift {
		// Get router TLS certificates chain
		peerCertificates, err := GetEndpointTLSCrtChain(checluster, "", clusterAPI)
		if err != nil {
			return false, err
		}

		// Check the chain if ti contains self-signed CA certificate
		for _, cert := range peerCertificates {
			if cert.Subject.String() == cert.Issuer.String() {
				// Self-signed CA certificate is found in the chain
				return true, nil
			}
		}
		// The chain doesn't contain self-signed certificate
		return false, nil
	}

	// For Kubernetes, check che-tls secret.

	cheTLSSecretName := checluster.Spec.K8s.TlsSecretName
	if len(cheTLSSecretName) < 1 {
		cheTLSSecretName = DefaultCheTLSSecretName
	}

	cheTLSSecret := &corev1.Secret{}
	err = clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: checluster.Namespace, Name: cheTLSSecretName}, cheTLSSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Failed to get secret, return error to restart reconcile loop.
			return false, err
		}

		// Both secrets (che-tls and self-signed-certificate) are missing which means that we should generate them (i.e. use self-signed certificate).
		return true, nil
	}
	// TLS secret found, consider it as commonly trusted.
	return false, nil
}

// GetEndpointTLSCrtChain retrieves TLS certificates chain from given endpoint.
// If endpoint is not specified, then a test route will be created and used to get router certificates.
func GetEndpointTLSCrtChain(instance *orgv1.CheCluster, endpointURL string, clusterAPI ClusterAPI) ([]*x509.Certificate, error) {
	if util.IsTestMode() {
		return nil, stderrors.New("Not allowed for tests")
	}

	var requestURL string

	if len(endpointURL) < 1 {
		var routeStatus RouteProvisioningStatus

		for wait := true; wait; {
			routeStatus = SyncRouteToCluster(instance, "test", "test", 8080, clusterAPI)
			if routeStatus.Err != nil {
				return nil, routeStatus.Err
			}
			wait = !routeStatus.Continue || len(routeStatus.Route.Spec.Host) == 0
			time.Sleep(time.Duration(1) * time.Second)
		}
		requestURL = "https://" + routeStatus.Route.Spec.Host

		// Cleanup route after the job done.
		defer func() {
			// Wait before deleting the route some time.
			// This is an optimization to avoid often route creation/deletion in case when required to test route certificates.
			// Mostly needed for case with commonly trusted cetificates.
			time.Sleep(time.Minute)
			if err := clusterAPI.Client.Delete(context.TODO(), routeStatus.Route); err != nil {
				if !errors.IsNotFound(err) {
					logrus.Errorf("Failed to delete test route %s: %s", routeStatus.Route.Name, err)
				}
			} else {
				logrus.Infof("Deleted a test route %s to extract routes crt", routeStatus.Route.Name)
			}
		}()

	} else {
		requestURL = endpointURL
	}

	// Adding the proxy settings to the Transport object
	transport := &http.Transport{}
	if instance.Spec.Server.ProxyURL != "" {
		logrus.Infof("Configuring proxy with %s to extract crt from the following URL: %s", instance.Spec.Server.ProxyURL, requestURL)
		configureProxy(instance, transport)
	}
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{
		Transport: transport,
	}

	req, err := http.NewRequest("GET", requestURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("An error occurred when reaching test TLS route: %s", err)
		return nil, err
	}

	return resp.TLS.PeerCertificates, nil
}

// GetEndpointTLSCrtBytes creates a test TLS route and gets it to extract certificate chain
// There's an easier way which is to read tls secret in default (3.11) or openshift-ingress (4.0) namespace
// which however requires extra privileges for operator service account
func GetEndpointTLSCrtBytes(instance *orgv1.CheCluster, endpointURL string, clusterAPI ClusterAPI) (certificates []byte, err error) {
	peerCertificates, err := GetEndpointTLSCrtChain(instance, endpointURL, clusterAPI)
	if err != nil {
		if util.IsTestMode() {
			fakeCrt := make([]byte, 5)
			return fakeCrt, nil
		}
		return nil, err
	}

	for i := range peerCertificates {
		cert := peerCertificates[i].Raw
		crt := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		})
		certificates = append(certificates, crt...)
	}

	return certificates, nil
}

func configureProxy(instance *orgv1.CheCluster, transport *http.Transport) {
	proxyParts := strings.Split(instance.Spec.Server.ProxyURL, "://")
	proxyProtocol := ""
	proxyHost := ""
	if len(proxyParts) == 1 {
		proxyProtocol = ""
		proxyHost = proxyParts[0]
	} else {
		proxyProtocol = proxyParts[0]
		proxyHost = proxyParts[1]

	}

	proxyURL := proxyHost
	if instance.Spec.Server.ProxyPort != "" {
		proxyURL = proxyURL + ":" + instance.Spec.Server.ProxyPort
	}

	proxyUser := instance.Spec.Server.ProxyUser
	proxyPassword := instance.Spec.Server.ProxyPassword
	proxySecret := instance.Spec.Server.ProxySecret
	user, password, err := util.K8sclient.ReadSecret(proxySecret, instance.Namespace)
	if err == nil {
		proxyUser = user
		proxyPassword = password
	} else {
		logrus.Errorf("Failed to read '%s' secret: %s", proxySecret, err)
	}
	if len(proxyUser) > 1 && len(proxyPassword) > 1 {
		proxyURL = proxyUser + ":" + proxyPassword + "@" + proxyURL
	}

	if proxyProtocol != "" {
		proxyURL = proxyProtocol + "://" + proxyURL
	}
	config := httpproxy.Config{
		HTTPProxy:  proxyURL,
		HTTPSProxy: proxyURL,
		NoProxy:    strings.Replace(instance.Spec.Server.NonProxyHosts, "|", ",", -1),
	}
	proxyFunc := config.ProxyFunc()
	transport.Proxy = func(r *http.Request) (*url.URL, error) {
		theProxyURL, err := proxyFunc(r.URL)
		if err != nil {
			logrus.Warnf("Error when trying to get the proxy to access TLS endpoint URL: %s - %s", r.URL, err)
		}
		logrus.Infof("Using proxy: %s to access TLS endpoint URL: %s", theProxyURL, r.URL)
		return theProxyURL, err
	}
}

// K8sHandleCheTLSSecrets handles TLS secrets required for Che deployment on Kubernetes infrastructure.
func K8sHandleCheTLSSecrets(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) (reconcile.Result, error) {
	cheTLSSecretName := checluster.Spec.K8s.TlsSecretName

	// ===== Check Che server TLS certificate ===== //

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
		sa, err := SyncServiceAccountToCluster(checluster, CheTLSJobServiceAccountName, clusterAPI)
		if sa == nil {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		role, err := SyncRoleToCluster(checluster, CheTLSJobRoleName, []string{"secrets"}, []string{"create"}, clusterAPI)
		if role == nil {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		roleBiding, err := SyncRoleBindingToCluster(checluster, CheTLSJobRoleBindingName, CheTLSJobServiceAccountName, CheTLSJobRoleName, "Role", clusterAPI)
		if roleBiding == nil {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		cheTLSSecretsCreationJobImage := DefaultCheTLSSecretsCreationJobImage()
		jobEnvVars := map[string]string{
			"DOMAIN":                         checluster.Spec.K8s.IngressDomain,
			"CHE_NAMESPACE":                  checluster.Namespace,
			"CHE_SERVER_TLS_SECRET_NAME":     cheTLSSecretName,
			"CHE_CA_CERTIFICATE_SECRET_NAME": CheTLSSelfSignedCertificateSecretName,
		}
		job, err := SyncJobToCluster(checluster, CheTLSJobName, CheTLSJobComponentName, cheTLSSecretsCreationJobImage, CheTLSJobServiceAccountName, jobEnvVars, clusterAPI)
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
		// This means that commonly trusted certificate is used.
	} else {
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
	}

	// TLS configuration is ok, go further in reconcile loop
	return reconcile.Result{}, nil
}

// CheckAndUpdateK8sTLSConfiguration validates Che TLS configuration on Kubernetes infra.
// If configuration is wrong it will guess most common use cases and will make changes in Che CR accordingly to the assumption.
func CheckAndUpdateK8sTLSConfiguration(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) error {
	if checluster.Spec.K8s.TlsSecretName == "" {
		checluster.Spec.K8s.TlsSecretName = DefaultCheTLSSecretName
		if err := UpdateCheCRSpec(checluster, "TlsSecretName", checluster.Spec.K8s.TlsSecretName, clusterAPI); err != nil {
			return err
		}
	}

	return nil
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

func deleteJob(job *batchv1.Job, checluster *orgv1.CheCluster, clusterAPI ClusterAPI) {
	names := util.K8sclient.GetPodsByComponent(CheTLSJobComponentName, checluster.Namespace)
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
