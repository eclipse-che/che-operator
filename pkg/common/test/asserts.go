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

package test

import (
	"context"
	"fmt"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func ReconcileTwice(
	cheCtx *chetypes.CheContext,
	reconcileFunc func(cheCtx *chetypes.CheContext) (result reconcile.Result, done bool, err error)) error {

	result, done, err := reconcileFunc(cheCtx)
	if err != nil {
		return fmt.Errorf("failed to reconcile on the first call: %w", err)
	}
	if done {
		if !reflect.DeepEqual(result, reconcile.Result{}) {
			return fmt.Errorf("reconciler is done but returned non-empty result: %+v", result)
		}
		return nil
	}

	result, done, err = reconcileFunc(cheCtx)
	if err != nil {
		return fmt.Errorf("failed to reconcile on the second call: %w", err)
	}
	if !done {
		return fmt.Errorf("reconciler is not done after the second call, result: %+v", result)
	}
	if !reflect.DeepEqual(result, reconcile.Result{}) {
		return fmt.Errorf("reconciler is done but returned non-empty result: %+v", result)
	}

	return nil
}

func ExpectFound(key client.ObjectKey, obj client.Object) error {
	err := clusterapi.Get().Client.Get(context.TODO(), key, obj)
	if k8serrors.IsNotFound(err) {
		return fmt.Errorf("expected %T %s to exist, but it was not found", obj, key)
	}
	if err != nil {
		return fmt.Errorf("failed to get %T %s: %w", obj, key, err)
	}

	return nil
}

func ExpectNotFound(key client.ObjectKey, obj client.Object) error {
	err := clusterapi.Get().Client.Get(context.TODO(), key, obj)
	if err == nil {
		return fmt.Errorf("expected %T %s to not exist, but it was found", obj, key)
	}

	if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get %T %s: %w", obj, key, err)
	}

	return nil
}
