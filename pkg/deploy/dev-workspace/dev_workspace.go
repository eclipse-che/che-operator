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

package devworkspace

import (
	"context"
	"os"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DevWorkspaceNamespace      = "devworkspace-controller"
	DevWorkspaceDeploymentName = "devworkspace-controller-manager"
	DevWorkspaceWebhookName    = "controller.devfile.io"

	SubscriptionResourceName          = "subscriptions"
	ClusterServiceVersionResourceName = "clusterserviceversions"
	DevWorkspaceCSVNamePrefix         = "devworkspace-operator"

	WebTerminalOperatorSubscriptionName = "web-terminal"

	OperatorNamespace = "openshift-operators"
)

var (
	// Exits the operator after successful fresh installation of the devworkspace.
	// Can be replaced with something less drastic (especially useful in tests)
	restartCheOperator = func() {
		logrus.Warn("Exiting the operator after DevWorkspace installation. DevWorkspace support will be initialized on the next start.")
		os.Exit(0)
	}
)

type DevWorkspaceReconciler struct {
	deploy.Reconcilable
}

func NewDevWorkspaceReconciler() *DevWorkspaceReconciler {
	return &DevWorkspaceReconciler{}
}

func (d *DevWorkspaceReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if isDevWorkspaceOperatorCSVExists(ctx) {
		// Do nothing if DevWorkspace has been already deployed via OLM
		return reconcile.Result{}, true, nil
	}

	if infrastructure.IsOpenShift() {
		wtoInstalled, err := isWebTerminalSubscriptionExist(ctx)
		if err != nil {
			return reconcile.Result{Requeue: true}, false, err
		}
		if wtoInstalled {
			// Do nothing if WTO exists since it should bring or embeds DWO
			return reconcile.Result{}, true, nil
		}
	}

	isCreated, err := createDwNamespace(ctx)
	if err != nil {
		return reconcile.Result{Requeue: true}, false, err
	}
	if !isCreated {
		namespace := &corev1.Namespace{}
		err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Name: DevWorkspaceNamespace}, namespace)
		if err != nil {
			return reconcile.Result{Requeue: true}, false, err
		}

		namespaceOwnershipAnnotation := namespace.GetAnnotations()[constants.CheEclipseOrgNamespace]
		if namespaceOwnershipAnnotation == "" {
			// don't manage DWO if namespace is create by someone else not but not Che Operator
			return reconcile.Result{}, true, nil
		}

		// if DWO is managed by another Che, check if we should take control under it after possible removal
		if namespaceOwnershipAnnotation != ctx.CheCluster.Namespace {
			isOnlyOneOperatorManagesDWResources, err := isOnlyOneOperatorManagesDWResources(ctx)
			if err != nil {
				return reconcile.Result{Requeue: true}, false, err
			}
			if !isOnlyOneOperatorManagesDWResources {
				// Don't take a control over DWO if CheCluster in another CR is handling it
				return reconcile.Result{}, true, nil
			}
			namespace.GetAnnotations()[constants.CheEclipseOrgNamespace] = ctx.CheCluster.Namespace
			_, err = deploy.Sync(ctx, namespace)
			if err != nil {
				return reconcile.Result{Requeue: true}, false, err
			}
		}
	}

	// check if DW exists on the cluster
	devWorkspaceWebHookExistedBeforeSync, err := deploy.Get(
		ctx,
		client.ObjectKey{Name: DevWorkspaceWebhookName},
		&admissionregistrationv1.MutatingWebhookConfiguration{},
	)

	for _, syncItem := range syncItems {
		_, err := syncItem(ctx)
		if err != nil {
			return reconcile.Result{Requeue: true}, false, err
		}
	}

	if !test.IsTestMode() {
		if !devWorkspaceWebHookExistedBeforeSync {
			// we need to restart Che Operator to switch devworkspace controller mode
			restartCheOperator()
		}
	}

	return reconcile.Result{}, true, nil
}

func (d *DevWorkspaceReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
