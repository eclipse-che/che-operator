package cheuser

import (
	"context"
	"sync"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	projectv1 "github.com/openshift/api/project/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ownerUidLabel            string = "che.eclipse.org/workspace-namespace-owner-uid"
	cheClusterNameLabel      string = "che.eclipse.org/che-name"
	cheClusterNamespaceLabel string = "che.eclipse.org/che-namespace"
)

type namespaceCache struct {
	client          client.Client
	knownNamespaces map[string]namespaceInfo
	lock            sync.Mutex
}

type namespaceInfo struct {
	OwnerUid   string
	CheCluster *types.NamespacedName
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
	for k, _ := range c.knownNamespaces {
		ret = append(ret, k)
	}

	return ret
}

func (c *namespaceCache) examineNamespaceUnsafe(ctx context.Context, ns string) (*namespaceInfo, error) {
	var obj runtime.Object
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

	ownerUid := labels[ownerUidLabel]
	cheName := labels[cheClusterNameLabel]
	cheNamespace := labels[cheClusterNamespaceLabel]

	ret := namespaceInfo{
		OwnerUid: ownerUid,
		CheCluster: &types.NamespacedName{
			Name:      cheName,
			Namespace: cheNamespace,
		},
	}

	c.knownNamespaces[ns] = ret

	return &ret, nil
}
