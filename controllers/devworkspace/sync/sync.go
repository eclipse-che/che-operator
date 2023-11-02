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

package sync

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	log = ctrl.Log.WithName("sync")
)

// Syncer synchronized K8s objects with the cluster
type Syncer struct {
	client client.Client
	scheme *runtime.Scheme
}

func New(client client.Client, scheme *runtime.Scheme) Syncer {
	return Syncer{client: client, scheme: scheme}
}

// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
// Returns true if the object was created or updated, false if there was no change detected.
func (s *Syncer) Sync(ctx context.Context, owner client.Object, blueprint client.Object, diffOpts cmp.Option) (bool, client.Object, error) {

	key := client.ObjectKey{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	actual := blueprint.DeepCopyObject().(client.Object)

	if getErr := s.client.Get(context.TODO(), key, actual); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return false, nil, getErr
		}
		actual = nil
	}

	if actual == nil {
		actual, err := s.create(ctx, owner, key, blueprint)
		if err != nil {
			return false, actual, err
		}

		return true, actual, nil
	}

	return s.update(ctx, owner, actual, blueprint, diffOpts)
}

// Delete deletes the supplied object from the cluster.
func (s *Syncer) Delete(ctx context.Context, object client.Object) error {
	key := client.ObjectKey{Name: object.GetName(), Namespace: object.GetNamespace()}

	var err error
	if err = s.client.Get(ctx, key, object); err == nil {
		err = s.client.Delete(ctx, object)
	}

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *Syncer) create(ctx context.Context, owner client.Object, key client.ObjectKey, blueprint client.Object) (client.Object, error) {
	actual := blueprint.DeepCopyObject().(client.Object)
	kind := deploy.GetObjectType(blueprint)
	log.Info("Creating a new object", "kind", kind, "name", blueprint.GetName(), "namespace", blueprint.GetNamespace())
	obj, err := s.setOwnerReference(owner, blueprint)
	if err != nil {
		return nil, err
	}

	err = s.client.Create(ctx, obj)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return nil, err
		}

		// ok, we got an already-exists error. So let's try to load the object into "actual".
		// if we fail this retry for whatever reason, just give up rather than retrying this in a loop...
		// the reconciliation loop will lead us here again in the next round.
		if err = s.client.Get(ctx, key, actual); err != nil {
			return nil, err
		}
	}

	return actual, nil
}

func (s *Syncer) update(ctx context.Context, owner client.Object, actual client.Object, blueprint client.Object, diffOpts cmp.Option) (bool, client.Object, error) {
	actualMeta := actual.(metav1.Object)

	diff := cmp.Diff(actual, blueprint, diffOpts)
	if len(diff) > 0 {
		kind := actual.GetObjectKind().GroupVersionKind().Kind
		log.Info("Updating existing object", "kind", kind, "name", actualMeta.GetName(), "namespace", actualMeta.GetNamespace())

		// we need to handle labels and annotations specially in case the cluster admin has modified them.
		// if the current object in the cluster has the same annos/labels, they get overwritten with what's
		// in the blueprint. Any additional labels/annos on the object are kept though.
		targetLabels := map[string]string{}
		targetAnnos := map[string]string{}

		for k, v := range actualMeta.GetAnnotations() {
			targetAnnos[k] = v
		}
		for k, v := range actualMeta.GetLabels() {
			targetLabels[k] = v
		}

		for k, v := range blueprint.GetAnnotations() {
			targetAnnos[k] = v
		}
		for k, v := range blueprint.GetLabels() {
			targetLabels[k] = v
		}

		blueprint.SetAnnotations(targetAnnos)
		blueprint.SetLabels(targetLabels)

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			err := s.client.Delete(ctx, actual)
			if err != nil {
				return false, actual, err
			}

			key := client.ObjectKey{Name: actualMeta.GetName(), Namespace: actualMeta.GetNamespace()}
			obj, err := s.create(ctx, owner, key, blueprint)
			return false, obj, err
		} else {
			obj, err := s.setOwnerReference(owner, blueprint)
			if err != nil {
				return false, actual, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			obj.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())

			err = s.client.Update(ctx, obj)
			if err != nil {
				return false, obj, err
			}

			return true, obj, nil
		}
	}
	return false, actual, nil
}

func isUpdateUsingDeleteCreate(kind string) bool {
	// Routes are not able to update the host, so we just need to re-create them...
	// ingresses and services have been identified to needs this, too, for reasons that I don't know..
	return "Service" == kind || "Ingress" == kind || "Route" == kind
}

func (s *Syncer) setOwnerReference(owner client.Object, obj client.Object) (client.Object, error) {
	if owner == nil {
		return obj, nil
	}

	if err := controllerutil.SetControllerReference(owner, obj, s.scheme); err != nil {
		return nil, err
	}

	return obj, nil
}
