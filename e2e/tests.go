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
package main

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/controller/che"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd/api"
	"log"
)

var (
	crName    = "eclipse-che"
	kind      = "checlusters"
	groupName = "org.eclipse.che"
	namespace = "che"
)

func main() {
	logrus.Info("Starting CHE/CRW operator e2e tests")
	logrus.Info("A running OCP instance and cluster-admin login are required")
	logrus.Info("Adding CRD to schema")
	if err := orgv1.SchemeBuilder.AddToScheme(scheme.Scheme); err != nil {
		logrus.Fatalf("Failed to add CRD to scheme")
	}
	apiScheme := runtime.NewScheme()
	if err := api.AddToScheme(apiScheme); err != nil {
		logrus.Fatalf("Failed to add CRD to scheme")
	}
	logrus.Info("CRD successfully added to schema")


	logrus.Infof("Creating a new namespace: %s", namespace)
	ns := newNamespace()
	if err := createNamespace(ns); err != nil {
		logrus.Fatalf("Failed to create a namespace %s: %s", ns.Name, err)
	}

	logrus.Info("Creating a new CR")
	err = createCR()
	if err != nil {
		logrus.Fatalf("Failed to create %s CR: %s", crName, err)
	}
	logrus.Info("CR has been successfully created")

	logrus.Infof("Getting CR %s to verify it has been successfully created", crName)
	cheCluster, err := getCR()
	if err != nil {
		logrus.Fatalf("An error occurred: %s", err)
	}
	logrus.Infof("CR found: name: %s", cheCluster.Name)

	logrus.Info("Creating a service account for operator deployment")

	operatorServiceAccount, err := deserializeOperatorServiceAccount()
	if err := createOperatorServiceAccount(operatorServiceAccount); err != nil {
		logrus.Fatalf("Failed to create Operator service account: %s", err)
	}

	logrus.Info("Creating role for operator service account")

	operatorServiceAccountRole, err := deserializeOperatorRole()
	if err := createOperatorServiceAccountRole(operatorServiceAccountRole); err != nil {
		logrus.Fatalf("Failed to create Operator service account role: %s", err)

	}

	logrus.Info("Creating RoleBinding")
	operatorServiceAccountRoleBinding, err := deserializeOperatorRoleBinding()
	if err := createOperatorServiceAccountRoleBinding(operatorServiceAccountRoleBinding); err != nil {
		logrus.Fatalf("Failed to create Operator service account role binding: %s", err)

	}

	logrus.Info("Deploying operator")
	operatorDeployment, err := deserializeOperatorDeployment()
	if err := deployOperator(operatorDeployment); err != nil {
		logrus.Fatalf("Failed to create Operator deployment: %s", err)
	}

	logrus.Info("Waiting for CR Available status. Timeout 15 min")
	deployed, err := VerifyCheRunning(che.AvailableStatus)
	if deployed {
		logrus.Info("Installation succeeded")
	}


	// reconfigure CR to enable TLS support
	logrus.Info("Patching CR with TLS enabled. This should cause a new Che deployment")
	patchPath := "/spec/server/tlsSupport"
	if err := patchCustomResource(patchPath, true); err != nil {
		logrus.Fatalf("An error occurred while patching CR %s", err)
	}

	// check if a CR status has changed to Rolling update in progress
	redeployed, err := VerifyCheRunning(che.RollingUpdateInProgressStatus)
	if redeployed {
		logrus.Info("New deployment triggered")
	}

	// wait for Available status
	logrus.Info("Waiting for CR Available status. Timeout 6 min")
	deployed, err = VerifyCheRunning(che.AvailableStatus)
	if deployed {
		logrus.Info("Installation succeeded")
	}

	// create clusterRole and clusterRoleBinding to let operator service account create oAuthclients
	logrus.Info("Creating cluster role for operator service account")

	operatorServiceAccountClusterRole, err := deserializeOperatorClusterRole()
	if err := createOperatorServiceAccountClusterRole(operatorServiceAccountClusterRole); err != nil {
		logrus.Fatalf("Failed to create Operator service account cluster role: %s", err)

	}

	logrus.Info("Creating RoleBinding")
	operatorServiceAccountClusterRoleBinding, err := deserializeOperatorClusterRoleBinding()
	if err := createOperatorServiceAccountClusterRoleBinding(operatorServiceAccountClusterRoleBinding); err != nil {
		logrus.Fatalf("Failed to create Operator service account cluster role binding: %s", err)

	}

	// reconfigure CR to enable login with OpenShift
	logrus.Info("Patching CR with oAuth enabled. This should cause a new Che deployment")
	patchPath = "/spec/auth/openShiftoAuth"
	if err := patchCustomResource(patchPath, true); err != nil {
		logrus.Fatalf("An error occurred while patching CR %s", err)
	}

	// check if a CR status has changed to Rolling update in progress
	redeployed, err = VerifyCheRunning(che.RollingUpdateInProgressStatus)
	if redeployed {
		logrus.Info("New deployment triggered")
	}

	// wait for Available status
	logrus.Info("Waiting for CR Available status. Timeout 15 min")
	deployed, err = VerifyCheRunning(che.AvailableStatus)
	if deployed {
		logrus.Info("Installation succeeded")
	}

	// check if oAuthClient has been created
	cr, err := getCR()
	if err != nil {
		logrus.Fatalf("Failed to get CR: %s", err)
	}
	oAuthClientName := cr.Spec.Auth.OauthClientName
	_, err = getOauthClient(oAuthClientName)
	if err != nil {
		logrus.Fatalf("oAuthclient %s not found", oAuthClientName)
	}
	logrus.Infof("Checking if oauthclient %s has been created", oAuthClientName)

	// verify oAuthClient name is set in che ConfigMap
	cm, err := getConfigMap("che")
	if err != nil {
		log.Fatalf("Failed to get ConfigMap: %s", err)
	}
	expectedIdentityProvider := "openshift-v3"
	actualIdentityProvider := cm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"]
	expectedWorkspaceProject := ""
	actualWorkspaceProject := cm.Data["CHE_INFRA_OPENSHIFT_PROJECT"]

	logrus.Info("Checking if identity provider is added to configmap")

	if expectedIdentityProvider != actualIdentityProvider {
		logrus.Fatalf("Test failed. Expecting identity provider: %s, got: %s", expectedIdentityProvider, actualIdentityProvider)
	}

	logrus.Info("Checking if workspace project is empty in CM")
	if expectedWorkspaceProject != actualWorkspaceProject {
		logrus.Fatalf("Test failed. Expecting identity provider: %s, got: %s", expectedWorkspaceProject, actualWorkspaceProject)
	}

	// cleanup
	logrus.Infof("Tests passed. Deleting namespace %s", namespace)
	if err := deleteNamespace(); err != nil {
		logrus.Errorf("Failed to delete namespace %s: %s", namespace, err)
	}
}
