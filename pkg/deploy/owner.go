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
package deploy

import (
	"context"
	stderrors "errors"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func HasCheClusterOwner(deployContext *DeployContext, object v1.Object) bool {
	for _, owner := range object.GetOwnerReferences() {
		if owner.Name == deployContext.CheCluster.Name {
			return true
		}
	}

	return false
}

func UpdateCheClusterOwner(deployContext *DeployContext, object v1.Object) error {
	if err := controllerutil.SetControllerReference(deployContext.CheCluster, object, deployContext.ClusterAPI.Scheme); err != nil {
		return err
	}

	robj, ok := object.(runtime.Object)
	if !ok {
		return stderrors.New("object " + object.GetName() + " is not a runtime.Object. Cannot update it")
	}

	return deployContext.ClusterAPI.Client.Update(context.TODO(), robj)
}
