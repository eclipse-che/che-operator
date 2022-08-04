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
	if infrastructure.IsOpenShift() {
		// Do nothing if Che operator has owner.
		// In this case DevWorkspace operator resources mustn't be managed by Che operator.
		cheOperatorHasOwner, err := isCheOperatorHasOwner(ctx)
		if cheOperatorHasOwner {
			return reconcile.Result{}, true, nil
		} else if err != nil {
			return reconcile.Result{Requeue: true}, false, err
		}

		// Do nothing if Dev Workspace operator has owner.
		// In this case DevWorkspace operator resources mustn't be managed by Che operator.
		devWorkspaceOperatorHasOwner, err := isDevWorkspaceOperatorHasOwner(ctx)
		if devWorkspaceOperatorHasOwner {
			return reconcile.Result{}, true, nil
		} else if err != nil {
			return reconcile.Result{Requeue: true}, false, err
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
			// don't manage DWO if namespace is created by someone else not but not Che Operator
			return reconcile.Result{}, true, nil
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
