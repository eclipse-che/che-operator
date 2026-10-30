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

package resources

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

var (
	CheCluster = cheClusterHandler{
		suppressDiff:       false,
		allowForceRecreate: false,
		diff: cmp.Options{
			cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "Status"),
		},
		statusDiff: cmp.Options{
			cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "ObjectMeta", "Spec"),
		},
	}
)

type cheClusterHandler struct {
	diff               cmp.Options
	statusDiff         cmp.Options
	suppressDiff       bool
	allowForceRecreate bool
}

func (ch cheClusterHandler) Find(
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
	}
	return nil, fmt.Errorf("found more than one CheCluster")
}

func (ch cheClusterHandler) AddFinalizer(cheCtx *chetypes.CheContext, finalizer string) error {
	if !controllerutil.AddFinalizer(cheCtx.CheCluster, finalizer) {
		return nil
	}
	return ch.Sync(cheCtx)
}

func (ch cheClusterHandler) RemoveAllFinalizers(cheCtx *chetypes.CheContext) error {
	if len(cheCtx.CheCluster.Finalizers) == 0 {
		return nil
	}
	cheCtx.CheCluster.Finalizers = nil
	return ch.Sync(cheCtx)
}

func (ch cheClusterHandler) Sync(cheCtx *chetypes.CheContext) error {
	return syncher.SyncObject(
		cheCtx,
		cheCtx.CheCluster,
		ch.diff,
		ch.suppressDiff,
		ch.allowForceRecreate,
	)
}

func (ch cheClusterHandler) SyncStatus(cheCtx *chetypes.CheContext) error {
	return syncher.SyncObjectStatus(
		cheCtx,
		cheCtx.CheCluster,
		ch.statusDiff,
		ch.suppressDiff,
	)
}
