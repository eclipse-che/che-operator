//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package checlusterbackup

import (
	"context"
	"strconv"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	InternalBackupServerType = "internal"

	BackupServerRepoPasswordSecretName = "backup-rest-server-repo-password"
	backupServerDeploymentName         = "backup-rest-server-deployment"
	backupServerPodName                = "backup-rest-server-pod"
	backupServerServiceName            = "backup-rest-server-service"
	backupServerPort                   = 8000
)

// ConfigureInternalBackupServer check for existance of internal REST backup server and deploys it if missing.
// If something is broken in the internal backup server configuration,
// then it will be recreated, but all data will be lost.
func ConfigureInternalBackupServer(bctx *BackupContext) (bool, error) {
	shouldInitResticRepo := false

	backupServerDeployment, err := getInternalBackupServerDeployment(bctx)
	if err != nil {
		return false, err
	}
	if backupServerDeployment == nil {
		shouldInitResticRepo = true
		err := createInternalBackupServerDeployment(bctx)
		if err != nil {
			return false, err
		}
	}

	err = ensureInternalBackupServerServiceExists(bctx)
	if err != nil {
		return false, err
	}

	err = ensureInternalBackupServerConfiguredAndCurrent(bctx)
	if err != nil {
		return false, err
	}

	// Check if the secret with restic repository password exists
	repoPasswordsecret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: bctx.backupCR.GetNamespace(),
		Name:      bctx.backupCR.Spec.Servers.Internal.RepoPasswordSecretRef,
	}
	err = bctx.r.client.Get(context.TODO(), namespacedName, repoPasswordsecret)
	if err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}

		if !shouldInitResticRepo {
			// Something is broken by a third party.
			// There is existing backup server, but no credentials to it.
			// The only way to regain access to the backup server is to completely recreate it.
			err := bctx.r.client.Delete(context.TODO(), backupServerDeployment)
			return false, err
		}

		// The secret with backup server credentials doesn't exist
		// Generate a new password and save it in the secret
		repoPassword := util.GeneratePasswd(12)
		repoPasswordsecret, err = getRepoPasswordSecretSpec(bctx, repoPassword)
		if err != nil {
			return false, err
		}
		err = bctx.r.client.Create(context.TODO(), repoPasswordsecret)
		if err != nil {
			return false, err
		}

		// Initialize new restic repository on the backup server
		return bctx.backupServer.InitRepository()
	}

	// The secret with backup server credentials exists
	// Check if the password is the right one
	done, err := bctx.backupServer.CheckRepository()
	if done && err != nil {
		// Check failed, the password is wrong
		// Clean broken stuff
		err = bctx.r.client.Delete(context.TODO(), repoPasswordsecret)
		if err != nil && !errors.IsNotFound(err) {
			return false, err
		}

		err = bctx.r.client.Delete(context.TODO(), repoPasswordsecret)
		if err != nil && !errors.IsNotFound(err) {
			return false, err
		}

		// Cleanup is done, but backup server should be created, request requeue
		return false, nil
	}
	return done, err
}

func getInternalBackupServerDeployment(bctx *BackupContext) (*appsv1.Deployment, error) {
	backupServerDeployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: bctx.backupCR.GetNamespace(),
		Name:      backupServerDeploymentName,
	}
	err := bctx.r.client.Get(context.TODO(), namespacedName, backupServerDeployment)
	if err == nil {
		return backupServerDeployment, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	return nil, nil
}

func createInternalBackupServerDeployment(bctx *BackupContext) error {
	// Get default configuration of the backup server deployment
	backupServerDeployment, err := getBackupServerDeploymentSpec(bctx)
	if err != nil {
		return err
	}
	// Create backup server deployment
	err = bctx.r.client.Create(context.TODO(), backupServerDeployment)
	if err != nil {
		return err
	}
	// Backup server created successfully
	return nil
}

func getBackupServerDeploymentSpec(bctx *BackupContext) (*appsv1.Deployment, error) {
	namespace := bctx.backupCR.GetNamespace()
	labels, labelSelector := deploy.GetLabelsAndSelector(bctx.cheCR, backupServerDeploymentName)

	replicas := int32(1)
	terminationGracePeriodSeconds := int64(30)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupServerDeploymentName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labelSelector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            backupServerPodName,
							Image:           "mm4eche/rest-server:latest", // TODO replace with official image
							ImagePullPolicy: "Always",
							Ports: []corev1.ContainerPort{
								{
									Name:          "rest",
									ContainerPort: backupServerPort,
									Protocol:      "TCP",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(backupServerPort),
										},
									},
								},
								InitialDelaySeconds: 1,
								FailureThreshold:    10,
								TimeoutSeconds:      1,
								SuccessThreshold:    1,
								PeriodSeconds:       1,
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	// Set CheClusterBackup instance as the owner and controller
	if err := controllerutil.SetControllerReference(bctx.backupCR, deployment, bctx.r.scheme); err != nil {
		return nil, err
	}

	return deployment, nil
}

func ensureInternalBackupServerServiceExists(bctx *BackupContext) error {
	backupServerService := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Namespace: bctx.backupCR.GetNamespace(),
		Name:      backupServerServiceName,
	}
	err := bctx.r.client.Get(context.TODO(), namespacedName, backupServerService)
	if err == nil {
		// Backup server service already exists, do nothing
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}

	// Backup server service doesn't exists, create it
	backupServerService, err = getBackupServerServiceSpec(bctx)
	if err != nil {
		return err
	}
	// Create backup server service
	err = bctx.r.client.Create(context.TODO(), backupServerService)
	if err != nil {
		return err
	}
	// Backup server service created successfully
	return nil
}

func getBackupServerServiceSpec(bctx *BackupContext) (*corev1.Service, error) {
	namespace := bctx.backupCR.GetNamespace()
	labels := deploy.GetLabels(bctx.cheCR, backupServerServiceName)

	port := corev1.ServicePort{
		Name:     backupServerServiceName + "-port",
		Port:     backupServerPort,
		Protocol: "TCP",
	}
	ports := []corev1.ServicePort{}
	ports = append(ports, port)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupServerServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}

	// Set CheClusterBackup instance as the owner and controller
	if err := controllerutil.SetControllerReference(bctx.backupCR, service, bctx.r.scheme); err != nil {
		return nil, err
	}

	return service, nil
}

// ensureInternalBackupServerConfiguredAndCurrent makes sure that current backup server is configured to internal rest server.
func ensureInternalBackupServerConfiguredAndCurrent(bctx *BackupContext) error {
	backupCR := bctx.backupCR

	shouldUpdateCR := false

	if backupCR.Spec.ServerType != InternalBackupServerType {
		backupCR.Spec.ServerType = InternalBackupServerType
		shouldUpdateCR = true
	}

	expectedInternalRestServerConfig := orgv1.RestServerConfing{
		Protocol: "http",
		Hostname: backupServerServiceName,
		Port:     strconv.Itoa(backupServerPort),
		Username: "user",
		Repo:     "che",
		RepoPassword: orgv1.RepoPassword{
			RepoPasswordSecretRef: BackupServerRepoPasswordSecretName,
		},
	}
	if backupCR.Spec.Servers.Internal != expectedInternalRestServerConfig {
		backupCR.Spec.Servers.Internal = expectedInternalRestServerConfig
		shouldUpdateCR = true
	}

	if shouldUpdateCR {
		err := bctx.r.UpdateCR(backupCR)
		if err != nil {
			return err
		}
	}

	return nil
}

func getRepoPasswordSecretSpec(bctx *BackupContext, password string) (*corev1.Secret, error) {
	namespace := bctx.backupCR.GetNamespace()
	labels := deploy.GetLabels(bctx.cheCR, BackupServerRepoPasswordSecretName)
	data := map[string][]byte{"repo-password": []byte(password)}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      BackupServerRepoPasswordSecretName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}

	if err := controllerutil.SetControllerReference(bctx.backupCR, secret, bctx.r.scheme); err != nil {
		return nil, err
	}

	return secret, nil
}
