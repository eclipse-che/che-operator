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

package usernamespace

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
	workspaceNamespaceOwnerUidLabel string = "che.eclipse.org/workspace-namespace-owner-uid"
	cheNameLabel                    string = "che.eclipse.org/che-name"
	cheNamespaceLabel               string = "che.eclipse.org/che-namespace"
	chePartOfLabel                  string = "app.kubernetes.io/part-of"
	chePartOfLabelValue             string = "che.eclipse.org"
	cheComponentLabel               string = "app.kubernetes.io/component"
	cheComponentLabelValue          string = "workspaces-namespace"
	cheUsernameAnnotation           string = "che.eclipse.org/username"
)

type namespaceCache struct {
	client          client.Client
	knownNamespaces map[string]namespaceInfo
	lock            sync.Mutex
}

type namespaceInfo struct {
	IsWorkspaceNamespace bool
	Username             string
	CheCluster           *types.NamespacedName
}

func NewNamespaceCache() *namespaceCache {
	return &namespaceCache{
		knownNamespaces: map[string]namespaceInfo{},
		lock:            sync.Mutex{},
	}
}

func (c *namespaceCache) GetNamespaceInfo(ctx context.Context, namespace string) (*namespaceInfo, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for {
		val, contains := c.knownNamespaces[namespace]
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
func (c *namespaceCache) ExamineNamespace(ctx context.Context, ns string) (*namespaceInfo, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.examineNamespaceUnsafe(ctx, ns)
}

func (c *namespaceCache) GetAllKnownNamespaces() []string {
	c.lock.Lock()
	defer c.lock.Unlock()

	ret := make([]string, len(c.knownNamespaces))
	i := 0
	for k := range c.knownNamespaces {
		ret[i] = k
		i = i + 1
	}

	return ret
}

func (c *namespaceCache) examineNamespaceUnsafe(ctx context.Context, ns string) (*namespaceInfo, error) {
	var obj client.Object
	if infrastructure.IsOpenShift() {
		obj = &projectv1.Project{}
	} else {
		obj = &corev1.Namespace{}
	}

	if err := c.client.Get(ctx, client.ObjectKey{Name: ns}, obj); err != nil {
		if errors.IsNotFound(err) {
			delete(c.knownNamespaces, ns)
			return nil, nil
		}
		return nil, err
	}

	var namespace = obj.(metav1.Object)

	if namespace.GetDeletionTimestamp() != nil {
		delete(c.knownNamespaces, ns)
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
	ownerUid := labels[workspaceNamespaceOwnerUidLabel]
	cheName := labels[cheNameLabel]
	cheNamespace := labels[cheNamespaceLabel]
	partOfLabel := labels[chePartOfLabel]
	componentLabel := labels[cheComponentLabel]
	username := annotations[cheUsernameAnnotation]

	ret := namespaceInfo{
		IsWorkspaceNamespace: ownerUid != "" || (partOfLabel == chePartOfLabelValue && componentLabel == cheComponentLabelValue),
		Username:             username,
		CheCluster: &types.NamespacedName{
			Name:      cheName,
			Namespace: cheNamespace,
		},
	}

	c.knownNamespaces[ns] = ret

	return &ret, nil
}
