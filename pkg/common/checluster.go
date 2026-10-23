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

package common

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

var (
	CheCluster = cheClusterHandler{
		logger: ctrl.Log.WithName("checluster"),
		diff: cmp.Options{
			cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "Status"),
		},
		statusDiff: cmp.Options{
			cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "ObjectMeta", "Spec"),
		},
	}
)

type cheClusterHandler struct {
	logger     logr.Logger
	diff       cmp.Options
	statusDiff cmp.Options
}

// FindSingleton returns the single CheCluster instance in the given namespace,
// or nil if none exists. An error is returned if multiple instances are found.
func (ch cheClusterHandler) FindSingleton(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
) (*chev2.CheCluster, error) {
	cheClusters := &chev2.CheClusterList{}
	if err := k8sClient.List(ctx, cheClusters, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list CheClusters in %s: %w", namespace, err)
	}

	switch len(cheClusters.Items) {
	case 0:
		return nil, nil
	case 1:
		return &cheClusters.Items[0], nil
	default:
		return nil, fmt.Errorf("expected one instance of CheCluster, but '%d' found", len(cheClusters.Items))
	}
}

// AddFinalizer adds the given finalizer to the CheCluster if not already present
// and syncs the update to the cluster.
func (ch cheClusterHandler) AddFinalizer(cheCtx *chetypes.CheContext, finalizer string) error {
	if !controllerutil.AddFinalizer(cheCtx.CheCluster, finalizer) {
		return nil
	}

	if err := ch.Sync(cheCtx); err != nil {
		return err
	}
	return ch.Refresh(cheCtx)
}

// RemoveAllFinalizers removes all finalizers from the CheCluster
// and syncs the update to the cluster.
func (ch cheClusterHandler) RemoveAllFinalizers(cheCtx *chetypes.CheContext) error {
	if len(cheCtx.CheCluster.Finalizers) == 0 {
		return nil
	}

	cheCtx.CheCluster.Finalizers = nil
	if err := ch.Sync(cheCtx); err != nil {
		return err
	}
	return ch.Refresh(cheCtx)
}

// UpdateStatus persists the CheCluster status subresource if it differs from
// the version currently stored in the cluster.
func (ch cheClusterHandler) UpdateStatus(cheCtx *chetypes.CheContext) (err error) {
	actualCheCluster := &chev2.CheCluster{}
	if err := ch.Get(cheCtx, actualCheCluster); err != nil {
		return err
	}

	diff := cmp.Diff(actualCheCluster, cheCtx.CheCluster, ch.statusDiff)
	if len(diff) == 0 {
		return nil
	}

	fmt.Printf("CheCluster difference:\n%s", diff)

	if err = cheCtx.ClusterAPI.Client.Status().Update(cheCtx.Context, cheCtx.CheCluster); err != nil {
		return fmt.Errorf("failed to update CheCluster: %w", err)
	}
	ch.logger.Info("Status updated")

	return ch.Refresh(cheCtx)
}

// Refresh updates the in-memory CheCluster held by the CheContext from the API server.
func (ch cheClusterHandler) Refresh(cheCtx *chetypes.CheContext) error {
	return ch.doGet(cheCtx, cheCtx.ClusterAPI.NonCachingClientWrapper, cheCtx.CheCluster)
}

// Get fetches a fresh copy of the CheCluster referenced by the given CheContext.
func (ch cheClusterHandler) Get(cheCtx *chetypes.CheContext, cheCluster *chev2.CheCluster) error {
	return ch.doGet(cheCtx, cheCtx.ClusterAPI.ClientWrapper, cheCluster)
}

// Sync persists the CheCluster spec and metadata if they differ from
// the version currently stored in the cluster.
func (ch cheClusterHandler) Sync(cheCtx *chetypes.CheContext) error {
	opts := &k8sclient.SyncOptions{
		DiffOpts:         ch.diff,
		MergeLabels:      true,
		MergeAnnotations: true,
	}

	if err := cheCtx.ClusterAPI.ClientWrapper.Sync(cheCtx.Context, cheCtx.CheCluster, opts); err != nil {
		return fmt.Errorf("failed to update CheCluster %s/%s: %w", cheCtx.CheCluster.Namespace, cheCtx.CheCluster.Name, err)
	}
	return nil
}

// doGet fetches the CheCluster referenced by the given CheContext
// using the provided client wrapper into the given object.
func (ch cheClusterHandler) doGet(cheCtx *chetypes.CheContext, cw *k8sclient.K8sClientWrapper, cheCluster *chev2.CheCluster) error {
	key := types.NamespacedName{
		Name:      cheCtx.CheCluster.Name,
		Namespace: cheCtx.CheCluster.Namespace,
	}

	if err := cw.Get(cheCtx.Context, key, cheCluster); err != nil {
		return fmt.Errorf("failed to get CheCluster %s: %w", key.String(), err)
	}

	return nil
}
