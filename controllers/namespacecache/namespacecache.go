//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package namespacecache

import (
	"context"
	"sync"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	projectv1 "github.com/openshift/api/project/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	WorkspaceNamespaceOwnerUidLabel string = "che.eclipse.org/workspace-namespace-owner-uid"
	CheNameLabel                    string = "che.eclipse.org/che-name"
	CheNamespaceLabel               string = "che.eclipse.org/che-namespace"
	ChePartOfLabel                  string = "app.kubernetes.io/part-of"
	ChePartOfLabelValue             string = "che.eclipse.org"
	CheComponentLabel               string = "app.kubernetes.io/component"
	CheComponentLabelValue          string = "workspaces-namespace"
	CheUsernameAnnotation           string = "che.eclipse.org/username"
)

type NamespaceCache struct {
	Client          client.Client
	KnownNamespaces map[string]NamespaceInfo
	Lock            sync.Mutex
}

type NamespaceInfo struct {
	IsWorkspaceNamespace bool
	Username             string
	CheCluster           *types.NamespacedName
}

func NewNamespaceCache(client client.Client) *NamespaceCache {
	return &NamespaceCache{
		Client:          client,
		KnownNamespaces: map[string]NamespaceInfo{},
		Lock:            sync.Mutex{},
	}
}

func (c *NamespaceCache) GetNamespaceInfo(ctx context.Context, namespace string) (*NamespaceInfo, error) {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	for {
		val, contains := c.KnownNamespaces[namespace]
		if contains {
			return &val, nil
		} else {
			existing, err := c.examineNamespaceUnsafe(ctx, namespace)
			if err != nil {
				return nil, err
			} else if existing == nil {
				return nil, nil
			}
		}
	}
}
func (c *NamespaceCache) ExamineNamespace(ctx context.Context, ns string) (*NamespaceInfo, error) {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	return c.examineNamespaceUnsafe(ctx, ns)
}

func (c *NamespaceCache) GetAllKnownNamespaces() []string {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	ret := make([]string, len(c.KnownNamespaces))
	i := 0
	for k := range c.KnownNamespaces {
		ret[i] = k
		i = i + 1
	}

	return ret
}

func (c *NamespaceCache) examineNamespaceUnsafe(ctx context.Context, ns string) (*NamespaceInfo, error) {
	var obj client.Object
	if infrastructure.IsOpenShift() {
		obj = &projectv1.Project{}
	} else {
		obj = &corev1.Namespace{}
	}

	if err := c.Client.Get(ctx, client.ObjectKey{Name: ns}, obj); err != nil {
		if errors.IsNotFound(err) {
			delete(c.KnownNamespaces, ns)
			return nil, nil
		}
		return nil, err
	}

	var namespace = obj.(metav1.Object)

	if namespace.GetDeletionTimestamp() != nil {
		delete(c.KnownNamespaces, ns)
		return nil, nil
	}

	labels := namespace.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	annotations := namespace.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// ownerUid is the legacy label that we used to use. Let's not break the existing workspace namespaces and still
	// recognize it
	ownerUid := labels[WorkspaceNamespaceOwnerUidLabel]
	cheName := labels[CheNameLabel]
	cheNamespace := labels[CheNamespaceLabel]
	partOfLabel := labels[ChePartOfLabel]
	componentLabel := labels[CheComponentLabel]
	username := annotations[CheUsernameAnnotation]

	ret := NamespaceInfo{
		IsWorkspaceNamespace: ownerUid != "" || (partOfLabel == ChePartOfLabelValue && componentLabel == CheComponentLabelValue),
		Username:             username,
		CheCluster: &types.NamespacedName{
			Name:      cheName,
			Namespace: cheNamespace,
		},
	}

	c.KnownNamespaces[ns] = ret

	return &ret, nil
}
