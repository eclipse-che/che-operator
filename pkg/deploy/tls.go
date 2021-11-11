//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
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

// IsSelfSignedCASecretExists checks if CheTLSSelfSignedCertificateSecretName exists so depending components can mount it
func IsSelfSignedCASecretExists(deployContext *DeployContext) (bool, error) {
	cheTLSSelfSignedCertificateSecret := &corev1.Secret{}
	err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: deployContext.CheCluster.Namespace, Name: CheTLSSelfSignedCertificateSecretName}, cheTLSSelfSignedCertificateSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// IsSelfSignedCertificateUsed detects whether endpoints are/should be secured by self-signed certificate.
func IsSelfSignedCertificateUsed(deployContext *DeployContext) (bool, error) {
	if util.IsTestMode() {
		return true, nil
	}

	cheCASecretExist, err := IsSelfSignedCASecretExists(deployContext)
	if err != nil {
		return false, err
	}

	if cheCASecretExist {
		return true, nil
	}

	if !util.IsOpenShift {
		// Handle custom tls secret for Che ingresses
		cheTLSSecretName := deployContext.CheCluster.Spec.K8s.TlsSecretName
		if cheTLSSecretName != "" {
			// The secret is specified in CR
			cheTLSSecret := &corev1.Secret{}
			err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: deployContext.CheCluster.Namespace, Name: cheTLSSecretName}, cheTLSSecret)
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

		// Empty secret name means using of default ingress certificate.
		// Retrieve the info about certificate chain from test ingress below.
	}

	// Get route/ingress TLS certificates chain
	peerCertificates, err := GetEndpointTLSCrtChain(deployContext, "")
	if err != nil {
		return false, err
	}

	// Check the chain if it contains self-signed CA certificate
	for _, cert := range peerCertificates {
		if cert.Subject.String() == cert.Issuer.String() {
			// Self-signed CA certificate is found in the chain
			return true, nil
		}
	}
	// The chain doesn't contain self-signed certificate
	return false, nil
}

// GetEndpointTLSCrtChain retrieves TLS certificates chain from given endpoint.
// If endpoint is not specified, then a test route/ingress will be created and used to get router certificates.
func GetEndpointTLSCrtChain(deployContext *DeployContext, endpointURL string) ([]*x509.Certificate, error) {
	if util.IsTestMode() {
		return nil, stderrors.New("Not allowed for tests")
	}

	var useTestEndpoint bool = len(endpointURL) < 1
	var requestURL string
	cheFlavor := DefaultCheFlavor(deployContext.CheCluster)
	if useTestEndpoint {
		if util.IsOpenShift {
			// Create test route to get certificates chain.
			// Note, it is not possible to use SyncRouteToCluster here as it may cause infinite reconcile loop.
			routeSpec, err := GetRouteSpec(
				deployContext,
				"test",
				"",
				"",
				"test",
				8080,
				deployContext.CheCluster.Spec.Server.CheServerRoute,
				cheFlavor)
			if err != nil {
				return nil, err
			}
			// Remove controller reference to prevent queueing new reconcile loop
			routeSpec.SetOwnerReferences(nil)
			// Create route manually
			if err := deployContext.ClusterAPI.Client.Create(context.TODO(), routeSpec); err != nil {
				if !errors.IsAlreadyExists(err) {
					logrus.Errorf("Failed to create test route 'test': %s", err)
					return nil, err
				}
			}

			// Schedule test route cleanup after the job done.
			defer func() {
				if err := deployContext.ClusterAPI.Client.Delete(context.TODO(), routeSpec); err != nil {
					logrus.Errorf("Failed to delete test route %s: %s", routeSpec.Name, err)
				}
			}()

			// Wait till the route is ready
			route := &routev1.Route{}
			for {
				time.Sleep(time.Duration(1) * time.Second)
				exists, err := GetNamespacedObject(deployContext, routeSpec.Name, route)
				if err != nil {
					return nil, err
				} else if exists {
					break
				}
			}

			requestURL = "https://" + route.Spec.Host
		} else {
			// Kubernetes

			// Create test ingress to get certificates chain.
			// Note, it is not possible to use SyncIngressToCluster here as it may cause infinite reconcile loop.
			_, ingressSpec := GetIngressSpec(
				deployContext,
				"test",
				"",
				"",
				"test",
				8080,
				deployContext.CheCluster.Spec.Server.CheServerIngress,
				cheFlavor)
			// Create ingress manually
			if err := deployContext.ClusterAPI.Client.Create(context.TODO(), ingressSpec); err != nil {
				if !errors.IsAlreadyExists(err) {
					logrus.Errorf("Failed to create test ingress 'test': %s", err)
					return nil, err
				}
			}

			// Schedule test ingress cleanup after the job done.
			defer func() {
				if err := deployContext.ClusterAPI.Client.Delete(context.TODO(), ingressSpec); err != nil {
					logrus.Errorf("Failed to delete test ingress %s: %s", ingressSpec.Name, err)
				}
			}()

			// Wait till the ingress is ready
			ingress := &networking.Ingress{}
			for {
				time.Sleep(time.Duration(1) * time.Second)
				exists, err := GetNamespacedObject(deployContext, ingressSpec.Name, ingress)
				if err != nil {
					return nil, err
				} else if exists {
					break
				}
			}

			requestURL = "https://" + ingress.Spec.Rules[0].Host
		}
	} else {
		requestURL = endpointURL
	}

	certificates, err := doRequestForTLSCrtChain(deployContext, requestURL, useTestEndpoint)
	if err != nil {
		if deployContext.Proxy.HttpProxy != "" && useTestEndpoint {
			// Fetching certificates from the test route without proxy failed. Probably non-proxy connections are blocked.
			// Retrying with proxy configuration, however it might cause retreiving of wrong certificate in case of TLS interception by proxy.
			logrus.Warn("Failed to get certificate chain of trust of the OpenShift Ingress bypassing the proxy")

			return doRequestForTLSCrtChain(deployContext, requestURL, false)
		}

		return nil, err
	}
	return certificates, nil
}

func doRequestForTLSCrtChain(deployContext *DeployContext, requestURL string, skipProxy bool) ([]*x509.Certificate, error) {
	transport := &http.Transport{}
	// Adding the proxy settings to the Transport object.
	// However, in case of test route we need to reach cluter directly in order to get the right certificate.
	if deployContext.Proxy.HttpProxy != "" && !skipProxy {
		logrus.Infof("Configuring proxy with %s to extract certificate chain from the following URL: %s", deployContext.Proxy.HttpProxy, requestURL)
		ConfigureProxy(deployContext, transport)
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

// GetEndpointTLSCrtBytes extracts certificate chain from given endpoint.
// Creates a test TLS route/ingress if endpoint url is empty.
// There's an easier way which is to read tls secret in default (3.11) or openshift-ingress (4.0) namespace
// which however requires extra privileges for operator service account
func GetEndpointTLSCrtBytes(deployContext *DeployContext, endpointURL string) (certificates []byte, err error) {
	peerCertificates, err := GetEndpointTLSCrtChain(deployContext, endpointURL)
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

// K8sHandleCheTLSSecrets handles TLS secrets required for Che deployment on Kubernetes infrastructure.
func K8sHandleCheTLSSecrets(deployContext *DeployContext) (reconcile.Result, error) {
	cheTLSSecretName := deployContext.CheCluster.Spec.K8s.TlsSecretName

	cheTLSSecretNamespacedName := types.NamespacedName{Namespace: deployContext.CheCluster.Namespace, Name: cheTLSSecretName}
	CheTLSSelfSignedCertificateSecretNamespacedName := types.NamespacedName{Namespace: deployContext.CheCluster.Namespace, Name: CheTLSSelfSignedCertificateSecretName}

	job := &batchv1.Job{}
	err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheTLSJobName, Namespace: deployContext.CheCluster.Namespace}, job)
	var jobExists bool
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		jobExists = false
	} else {
		jobExists = true
	}

	// ===== Check Che server TLS certificate ===== //

	cheTLSSecret := &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), cheTLSSecretNamespacedName, cheTLSSecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			// Error reading secret info
			logrus.Errorf("Error getting Che TLS secert \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		// Check if a job is already running for TLS secrets creation
		if jobExists {
			if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
				logrus.Infof("Waiting on job '%s' to be finished", CheTLSJobName)
				return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
			} else if job.Status.Succeeded > 0 {
				// Secrets are ready, restart reconcilation loop
				return reconcile.Result{}, nil
			}
		}

		// Che TLS secret doesn't exist, generate a new one

		// Remove Che CA certificate secret if any
		cheCASelfSignedCertificateSecret := &corev1.Secret{}
		err = deployContext.ClusterAPI.Client.Get(context.TODO(), CheTLSSelfSignedCertificateSecretNamespacedName, cheCASelfSignedCertificateSecret)
		if err != nil {
			if !errors.IsNotFound(err) {
				// Error reading secret info
				logrus.Errorf("Error getting Che self-signed certificate secert \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
			// Che CA certificate doesn't exists (that's expected at this point), do nothing
		} else {
			// Remove Che CA secret because Che TLS secret is missing (they should be generated together).
			if err = deployContext.ClusterAPI.Client.Delete(context.TODO(), cheCASelfSignedCertificateSecret); err != nil {
				logrus.Errorf("Error deleting Che self-signed certificate secret \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
		}

		// Prepare permissions for the certificate generation job
		done, err := SyncServiceAccountToCluster(deployContext, CheTLSJobServiceAccountName)
		if !done {
			return reconcile.Result{RequeueAfter: time.Second}, err
		}

		done, err = SyncTLSRoleToCluster(deployContext)
		if !done {
			return reconcile.Result{}, err
		}

		done, err = SyncRoleBindingToCluster(deployContext, CheTLSJobRoleBindingName, CheTLSJobServiceAccountName, CheTLSJobRoleName, "Role")
		if !done {
			return reconcile.Result{}, err
		}

		domains := deployContext.CheCluster.Spec.K8s.IngressDomain + ",*." + deployContext.CheCluster.Spec.K8s.IngressDomain
		if deployContext.CheCluster.Spec.Server.CheHost != "" && !strings.Contains(deployContext.CheCluster.Spec.Server.CheHost, deployContext.CheCluster.Spec.K8s.IngressDomain) && deployContext.CheCluster.Spec.Server.CheHostTLSSecret == "" {
			domains += "," + deployContext.CheCluster.Spec.Server.CheHost
		}

		labels := ""
		for labelName, labelValue := range GetLabels(deployContext.CheCluster, cheTLSSecretName) {
			labels += fmt.Sprintf("%s=%s ", labelName, labelValue)
		}

		cheTLSSecretsCreationJobImage := DefaultCheTLSSecretsCreationJobImage()
		jobEnvVars := map[string]string{
			"DOMAIN":                         domains,
			"CHE_NAMESPACE":                  deployContext.CheCluster.Namespace,
			"CHE_SERVER_TLS_SECRET_NAME":     cheTLSSecretName,
			"CHE_CA_CERTIFICATE_SECRET_NAME": CheTLSSelfSignedCertificateSecretName,
			"LABELS":                         labels,
		}

		_, err = SyncJobToCluster(deployContext, CheTLSJobName, CheTLSJobComponentName, cheTLSSecretsCreationJobImage, CheTLSJobServiceAccountName, jobEnvVars)
		if err != nil {
			logrus.Error(err)
		}
		return reconcile.Result{RequeueAfter: time.Second}, err
	}

	// cleanup job
	if jobExists {
		// The job object is present
		if job.Status.Succeeded > 0 {
			logrus.Infof("Import public part of Eclipse Che self-signed CA certificate from \"%s\" secret into your browser.", CheTLSSelfSignedCertificateSecretName)
			deleteJob(deployContext, job)
		} else if job.Status.Failed > 0 {
			// The job failed, but the certificate is present, shouldn't happen
			deleteJob(deployContext, job)
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
		if err = deployContext.ClusterAPI.Client.Delete(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		// Recreate the secret
		return reconcile.Result{RequeueAfter: time.Second}, err
	}

	// Check owner reference
	if cheTLSSecret.ObjectMeta.OwnerReferences == nil {
		// Set owner Che cluster as Che TLS secret owner
		if err := controllerutil.SetControllerReference(deployContext.CheCluster, cheTLSSecret, deployContext.ClusterAPI.Scheme); err != nil {
			logrus.Errorf("Failed to set owner for Che TLS secret \"%s\". Error: %s", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
		if err := deployContext.ClusterAPI.Client.Update(context.TODO(), cheTLSSecret); err != nil {
			logrus.Errorf("Failed to update owner for Che TLS secret \"%s\". Error: %s", cheTLSSecretName, err)
			return reconcile.Result{RequeueAfter: time.Second}, err
		}
	}

	// ===== Check Che CA certificate ===== //

	cheTLSSelfSignedCertificateSecret := &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), CheTLSSelfSignedCertificateSecretNamespacedName, cheTLSSelfSignedCertificateSecret)
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
			if err = deployContext.ClusterAPI.Client.Delete(context.TODO(), cheTLSSelfSignedCertificateSecret); err != nil {
				logrus.Errorf("Error deleting Che self-signed certificate secret \"%s\": %v", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
			// Also delete Che TLS as the certificates should be created together
			// Here it is not mandatory to check Che TLS secret existence as it is handled above
			if err = deployContext.ClusterAPI.Client.Delete(context.TODO(), cheTLSSecret); err != nil {
				logrus.Errorf("Error deleting Che TLS secret \"%s\": %v", cheTLSSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
			// Regenerate Che TLS certicates and recreate secrets
			return reconcile.Result{RequeueAfter: time.Second}, nil
		}

		// Check owner reference
		if cheTLSSelfSignedCertificateSecret.ObjectMeta.OwnerReferences == nil {
			// Set owner Che cluster as Che TLS secret owner
			if err := controllerutil.SetControllerReference(deployContext.CheCluster, cheTLSSelfSignedCertificateSecret, deployContext.ClusterAPI.Scheme); err != nil {
				logrus.Errorf("Failed to set owner for Che self-signed certificate secret \"%s\". Error: %s", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
			if err := deployContext.ClusterAPI.Client.Update(context.TODO(), cheTLSSelfSignedCertificateSecret); err != nil {
				logrus.Errorf("Failed to update owner for Che self-signed certificate secret \"%s\". Error: %s", CheTLSSelfSignedCertificateSecretName, err)
				return reconcile.Result{RequeueAfter: time.Second}, err
			}
		}
	}

	// TLS configuration is ok, go further in reconcile loop
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

func deleteJob(deployContext *DeployContext, job *batchv1.Job) {
	names := util.K8sclient.GetPodsByComponent(CheTLSJobComponentName, deployContext.CheCluster.Namespace)
	for _, podName := range names {
		pod := &corev1.Pod{}
		err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: deployContext.CheCluster.Namespace}, pod)
		if err == nil {
			// Delete pod (for some reasons pod isn't removed when job is removed)
			if err = deployContext.ClusterAPI.Client.Delete(context.TODO(), pod); err != nil {
				logrus.Errorf("Error deleting pod: '%s', error: %v", podName, err)
			}
		}
	}

	if err := deployContext.ClusterAPI.Client.Delete(context.TODO(), job); err != nil {
		logrus.Errorf("Error deleting job: '%s', error: %v", CheTLSJobName, err)
	}
}
