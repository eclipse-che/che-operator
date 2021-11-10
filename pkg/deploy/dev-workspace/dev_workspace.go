//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspace

import (
	"context"
	"fmt"
	"os"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	DevWorkspaceNamespace      = "devworkspace-controller"
	DevWorkspaceDeploymentName = "devworkspace-controller-manager"
	DevWorkspaceWebhookName    = "controller.devfile.io"

	SubscriptionResourceName          = "subscriptions"
	ClusterServiceVersionResourceName = "clusterserviceversions"
	DevWorkspaceCSVNamePrefix         = "devworkspace-operator"

	WebTerminalOperatorSubscriptionName = "web-terminal"
	WebTerminalOperatorNamespace        = "openshift-operators"
)

var (
	// Exits the operator after successful fresh installation of the devworkspace.
	// Can be replaced with something less drastic (especially useful in tests)
	restartCheOperator = func() {
		logrus.Warn("Exiting the operator after DevWorkspace installation. DevWorkspace support will be initialized on the next start.")
		os.Exit(0)
	}
)

func ReconcileDevWorkspace(deployContext *deploy.DeployContext) (done bool, err error) {
	if util.IsOpenShift && !util.IsOpenShift4 {
		// OpenShift 3.x is not supported
		return true, nil
	}

	if !deployContext.CheCluster.Spec.DevWorkspace.Enable {
		// Do nothing if DevWorkspace is disabled
		return true, nil
	}

	if isDevWorkspaceOperatorCSVExists(deployContext) {
		// Do nothing if DevWorkspace has been already deployed via OLM
		return true, nil
	}

	if util.IsOpenShift {
		//
		wtoInstalled, err := doesWebTerminalSubscriptionExist(deployContext)
		if err != nil {
			return false, err
		}
		if wtoInstalled {
			// Do nothing if WTO exists since it should bring or embeds DWO
			return true, nil
		}
	}

	if !deploy.IsDevWorkspaceEngineAllowed() {
		// Note: When the tech-preview-stable-all-namespaces will be by default stable-all-namespaces 7.40.0?, change the channel from the log
		exists, err := isDevWorkspaceDeploymentExists(deployContext)
		if err != nil {
			return false, err
		}
		if !exists {
			// Don't allow to deploy a new DevWorkspace operator
			return false, fmt.Errorf("To enable DevWorkspace engine, deploy Eclipse Che from tech-preview channel.")
		}

		// Allow existed Eclipse Che and DevWorkspace deployments to work
		// event though is not allowed (for backward compatibility)
		logrus.Warnf("To enable DevWorkspace engine, deploy Eclipse Che from tech-preview channel.")
	}

	isCreated, err := createDwNamespace(deployContext)
	if err != nil {
		return false, err
	}
	if !isCreated {
		namespace := &corev1.Namespace{}
		err := deployContext.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Name: DevWorkspaceNamespace}, namespace)
		if err != nil {
			return false, err
		}

		namespaceOwnershipAnnotation := namespace.GetAnnotations()[deploy.CheEclipseOrgNamespace]
		if namespaceOwnershipAnnotation == "" {
			// don't manage DWO if namespace is create by someone else not but not Che Operator
			return true, err
		}

		// if DWO is managed by another Che, check if we should take control under it after possible removal
		if namespaceOwnershipAnnotation != deployContext.CheCluster.Namespace {
			isOnlyOneOperatorManagesDWResources, err := isOnlyOneOperatorManagesDWResources(deployContext)
			if err != nil {
				return false, err
			}
			if !isOnlyOneOperatorManagesDWResources {
				// Don't take a control over DWO if CheCluster in another CR is handling it
				return true, nil
			}
			namespace.GetAnnotations()[deploy.CheEclipseOrgNamespace] = deployContext.CheCluster.Namespace
			_, err = deploy.Sync(deployContext, namespace)
			if err != nil {
				return false, err
			}
		}
	}

	// check if DW exists on the cluster
	devWorkspaceWebHookExistedBeforeSync, err := deploy.Get(
		deployContext,
		client.ObjectKey{Name: DevWorkspaceWebhookName},
		&admissionregistrationv1.MutatingWebhookConfiguration{},
	)

	for _, syncItem := range syncItems {
		_, err := syncItem(deployContext)
		if err != nil {
			return false, err
		}
	}

	if !util.IsTestMode() {
		if !devWorkspaceWebHookExistedBeforeSync {
			// we need to restart Che Operator to switch devworkspace controller mode
			restartCheOperator()
		}
	}

	return true, nil
}

func isDevWorkspaceDeploymentExists(deployContext *deploy.DeployContext) (bool, error) {
	return deploy.Get(deployContext, types.NamespacedName{
		Namespace: DevWorkspaceNamespace,
		Name:      DevWorkspaceDeploymentName,
	}, &appsv1.Deployment{})
}

func isDevWorkspaceOperatorCSVExists(deployContext *deploy.DeployContext) bool {
	// If clusterserviceversions resource doesn't exist in cluster DWO as well will not be present
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, ClusterServiceVersionResourceName) {
		return false
	}

	csvList := &operatorsv1alpha1.ClusterServiceVersionList{}
	err := deployContext.ClusterAPI.Client.List(context.TODO(), csvList, &client.ListOptions{})
	if err != nil {
		return false
	}

	for _, csv := range csvList.Items {
		if strings.HasPrefix(csv.Name, DevWorkspaceCSVNamePrefix) {
			return true
		}
	}

	return false
}

func doesWebTerminalSubscriptionExist(deployContext *deploy.DeployContext) (bool, error) {
	// If subscriptions resource doesn't exist in cluster WTO as well will not be present
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, SubscriptionResourceName) {
		return false, nil
	}

	subscription := &operatorsv1alpha1.Subscription{}
	if err := deployContext.ClusterAPI.NonCachedClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		subscription); err != nil {

		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func createDwNamespace(deployContext *deploy.DeployContext) (bool, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Spec: corev1.NamespaceSpec{},
	}

	return deploy.CreateIfNotExists(deployContext, namespace)
}

func isOnlyOneOperatorManagesDWResources(deployContext *deploy.DeployContext) (bool, error) {
	cheClusters := &orgv1.CheClusterList{}
	err := deployContext.ClusterAPI.NonCachedClient.List(context.TODO(), cheClusters)
	if err != nil {
		return false, err
	}

	devWorkspaceEnabledNum := 0
	for _, cheCluster := range cheClusters.Items {
		if cheCluster.Spec.DevWorkspace.Enable {
			devWorkspaceEnabledNum++
		}
	}

	return devWorkspaceEnabledNum == 1, nil
}
