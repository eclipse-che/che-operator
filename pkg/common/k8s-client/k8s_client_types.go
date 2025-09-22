//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//
//	Red Hat, Inc. - initial API and implementation

package k8s_client

import (
	"context"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClient interface {
	// Sync ensures that the object is up to date in the cluster.
	// Object is created if it does not exist and updated if it exists but is different.
	// Returns nil if object is in sync.
	Sync(ctx context.Context, blueprint client.Object, owner metav1.Object, opts ...SyncOption) error
	// Create creates object.
	// Returns nil if object is created otherwise returns error.
	Create(ctx context.Context, blueprint client.Object, owner metav1.Object, opts ...client.CreateOption) error
	// GetIgnoreNotFound gets object.
	// Returns true if object exists otherwise returns false.
	// Returns nil if object is retrieved or not found otherwise returns error.
	GetIgnoreNotFound(ctx context.Context, key client.ObjectKey, objectMeta client.Object, opts ...client.GetOption) (bool, error)
	// DeleteByKeyIgnoreNotFound deletes object by key.
	// Returns nil if object is deleted or not found otherwise returns error.
	DeleteByKeyIgnoreNotFound(ctx context.Context, key client.ObjectKey, objectMeta client.Object, opts ...client.DeleteOption) error
	// List returns list of runtime objects.
	// Returns nil if list is retrieved otherwise returns error.
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) ([]runtime.Object, error)
}

type SyncOption interface {
	ApplyToList(*SyncOptions)
}

type SyncOptions struct {
	// MergeLabels can be used to merge labels from existing object to the new one
	MergeLabels bool
	// MergeAnnotations can be used to merge annotations from existing object to the new one
	MergeAnnotations bool
	// SuppressDiff can be used to suppress printing diff when object is not in sync
	SuppressDiff bool
	// DiffOpts can be used to customize comparison when object is not in sync
	DiffOpts []cmp.Option
}

func (o *SyncOptions) ApplyToList(so *SyncOptions) {
	if o.MergeLabels {
		so.MergeLabels = o.MergeLabels
	}

	if o.MergeAnnotations {
		so.MergeAnnotations = o.MergeAnnotations
	}

	if o.SuppressDiff {
		so.SuppressDiff = o.SuppressDiff
	}

	if len(o.DiffOpts) == 0 {
		so.DiffOpts = o.DiffOpts
	}
}

func (o *SyncOptions) ApplyOptions(opts []SyncOption) {
	for _, opt := range opts {
		opt.ApplyToList(o)
	}
}
