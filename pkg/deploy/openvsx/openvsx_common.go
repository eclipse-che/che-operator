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

package openvsx

import (
	"context"
	"fmt"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func GetOpenVSXServerServiceURL(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf("http://%s.%s.svc:%d",
		constants.OpenVSXServerComponentName,
		ctx.CheCluster.Namespace,
		constants.OpenVSXServerServicePort,
	)
}

func EnsurePreviousNotExists(ctx *chetypes.DeployContext, name string) (reconcile.Result, bool, error) {
	jobKey := types.NamespacedName{
		Name:      name,
		Namespace: ctx.CheCluster.Namespace,
	}

	job := &batchv1.Job{}
	exits, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(context.TODO(), jobKey, job)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to get job %s: %w", job.Name, err)
	}
	if exits {
		if job.Status.Active != 0 {
			// Still in progress
			return reconcile.Result{}, true, nil
		}

		err = ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
			context.TODO(),
			jobKey,
			&batchv1.Job{},
			client.PropagationPolicy(metav1.DeletePropagationBackground),
		)
		if err != nil {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, false, fmt.Errorf("failed to delete job %s: %w", job.Name, err)
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, false, err
	}

	return reconcile.Result{}, true, nil
}
