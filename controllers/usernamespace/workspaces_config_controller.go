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
	"fmt"
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

const (
	syncedWorkspacesConfig = "synced-workspaces-config"
)

var (
	log                      = ctrl.Log.WithName("workspaces-config")
	workspacesConfigSelector = labels.SelectorFromSet(map[string]string{
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
	})
)

type WorkspacesConfigReconciler struct {
	scheme          *runtime.Scheme
	client          client.Client
	nonCachedClient client.Client
	namespaceCache  *namespaceCache
}

func NewWorkspacesConfigReconciler(
	client client.Client,
	noncachedClient client.Client,
	scheme *runtime.Scheme,
	namespaceCache *namespaceCache) *WorkspacesConfigReconciler {

	return &WorkspacesConfigReconciler{
		scheme:          scheme,
		client:          client,
		nonCachedClient: noncachedClient,
		namespaceCache:  namespaceCache,
	}
}

func (r *WorkspacesConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, r.watchRules(ctx)).
		Watches(&source.Kind{Type: &corev1.Secret{}}, r.watchRules(ctx)).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, r.watchRules(ctx))

	return bld.Complete(r)
}

func (r *WorkspacesConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	info, err := r.namespaceCache.ExamineNamespace(ctx, req.Name)
	if err != nil {
		log.Error(err, "Failed to examine namespace", "namespace", req.Name)
		return ctrl.Result{}, err
	}

	if info == nil || !info.IsWorkspaceNamespace {
		// namespace is not a workspace namespace, nothing to do
		return ctrl.Result{}, nil
	}

	checluster, err := deploy.FindCheClusterCRInNamespace(r.nonCachedClient, "")
	if checluster == nil || err != nil {
		// CheCluster is not found or error occurred, requeue the request
		return ctrl.Result{}, err
	}

	deployContext := &chetypes.DeployContext{
		CheCluster: checluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:           r.client,
			NonCachingClient: r.nonCachedClient,
			Scheme:           r.scheme,
		},
	}

	if err = r.syncWorkspacesConfig(ctx, req.Name, deployContext); err != nil {
		log.Error(err, "Failed to sync workspace configs", "namespace", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WorkspacesConfigReconciler) watchRules(ctx context.Context) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			return asReconcileRequestsForNamespaces(obj,
				[]eventRule{
					{
						// reconcile rule when workspace config is modified in a user namespace
						// to revert the config
						check: func(o metav1.Object) bool {
							workspaceInfo, _ := r.namespaceCache.GetNamespaceInfo(ctx, o.GetNamespace())
							return isLabeledAsWorkspacesConfig(o) &&
								o.GetName() != syncedWorkspacesConfig &&
								workspaceInfo != nil &&
								workspaceInfo.IsWorkspaceNamespace
						},
						namespaces: func(o metav1.Object) []string { return []string{o.GetNamespace()} },
					},
					{
						// reconcile rule when workspace config is modified in a che namespace
						// to update the config in all users` namespaces
						check: func(o metav1.Object) bool {
							cheCluster, _ := deploy.FindCheClusterCRInNamespace(r.nonCachedClient, o.GetNamespace())
							return isLabeledAsWorkspacesConfig(o) && cheCluster != nil
						},
						namespaces: func(o metav1.Object) []string { return r.namespaceCache.GetAllKnownNamespaces() },
					}})
		})
}

func isLabeledAsWorkspacesConfig(obj metav1.Object) bool {
	return obj.GetLabels()[constants.KubernetesComponentLabelKey] == constants.WorkspacesConfig
}

func (r *WorkspacesConfigReconciler) syncWorkspacesConfig(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext) error {
	syncedConfig, err := getSyncedConfig(ctx, targetNs, deployContext)
	if err != nil {
		log.Error(err, "Failed to get synced config", "namespace", targetNs)
		return nil
	}

	defer func() {
		if syncedConfig != nil {
			if syncedConfig.GetResourceVersion() == "" {
				if err := deployContext.ClusterAPI.NonCachingClient.Create(ctx, syncedConfig); err != nil {
					log.Error(err, "Failed to create synced config", "namespace", targetNs)
				}
			} else {
				if err := deployContext.ClusterAPI.NonCachingClient.Update(ctx, syncedConfig); err != nil {
					log.Error(err, "Failed to update synced config", "namespace", targetNs)
				}
			}
		}
	}()

	if err := syncConfigMaps(ctx, targetNs, deployContext, syncedConfig.Data); err != nil {
		return err
	}

	if err := syncSecrets(ctx, targetNs, deployContext, syncedConfig.Data); err != nil {
		return err
	}

	if err := syncPVC(ctx, targetNs, deployContext, syncedConfig.Data); err != nil {
		return err
	}

	return nil
}

func syncConfigMaps(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext, syncedData map[string]string) error {
	objs := &corev1.ConfigMapList{}
	if err := readObjects2Sync(ctx, deployContext, objs); err != nil {
		return err
	}

	for _, srcObj := range objs.Items {
		dstObj := srcObj.DeepCopy()
		dstObj.ObjectMeta = metav1.ObjectMeta{
			Name:        dstObj.Name,
			Namespace:   targetNs,
			Annotations: dstObj.Annotations,
			Labels:      dstObj.Labels,
		}
		dstObj.Labels[dwconstants.DevWorkspaceWatchConfigMapLabel] = "true"
		dstObj.Labels[dwconstants.DevWorkspaceMountLabel] = "true"
		addDefaultLabels(dstObj.Labels)

		if err := syncObject(ctx, deployContext, &srcObj, dstObj, syncedData); err != nil {
			log.Error(err, "Failed to sync ConfigMap", "namespace", dstObj.Namespace, "name", dstObj.Name)
		}
	}

	return nil
}

func syncSecrets(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext, syncedData map[string]string) error {
	objs := &corev1.SecretList{}
	if err := readObjects2Sync(ctx, deployContext, objs); err != nil {
		return err
	}

	for _, srcObj := range objs.Items {
		dstObj := srcObj.DeepCopy()
		dstObj.ObjectMeta = metav1.ObjectMeta{
			Name:        dstObj.Name,
			Namespace:   targetNs,
			Annotations: dstObj.Annotations,
			Labels:      dstObj.Labels,
		}
		dstObj.Labels[dwconstants.DevWorkspaceWatchSecretLabel] = "true"
		dstObj.Labels[dwconstants.DevWorkspaceMountLabel] = "true"
		addDefaultLabels(dstObj.Labels)

		if err := syncObject(ctx, deployContext, &srcObj, dstObj, syncedData); err != nil {
			log.Error(err, "Failed to sync Secret", "namespace", dstObj.Namespace, "name", dstObj.Name)
		}
	}

	return nil
}

func syncPVC(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext, syncedData map[string]string) error {
	objs := &corev1.PersistentVolumeClaimList{}
	if err := readObjects2Sync(ctx, deployContext, objs); err != nil {
		return err
	}

	for _, srcObj := range objs.Items {
		dstObj := srcObj.DeepCopy()
		dstObj.Status = corev1.PersistentVolumeClaimStatus{}
		dstObj.ObjectMeta = metav1.ObjectMeta{
			Name:        dstObj.Name,
			Namespace:   targetNs,
			Annotations: dstObj.Annotations,
			Labels:      dstObj.Labels,
		}
		addDefaultLabels(dstObj.Labels)

		if err := syncObject(ctx, deployContext, &srcObj, dstObj, syncedData); err != nil {
			log.Error(err, "Failed to sync PersistentVolumeClaim", "namespace", dstObj.Namespace, "name", dstObj.Name)
		}
	}

	return nil

}

func readObjects2Sync(ctx context.Context, deployContext *chetypes.DeployContext, objList client.ObjectList) error {
	return deployContext.ClusterAPI.Client.List(
		ctx,
		objList,
		&client.ListOptions{
			Namespace:     deployContext.CheCluster.Namespace,
			LabelSelector: workspacesConfigSelector,
		})
}

// syncObject syncs source object to destination object if they differ.
// Returns error if sync failed in a destination namespace.
func syncObject(
	ctx context.Context,
	deployContext *chetypes.DeployContext,
	srcObj client.Object,
	dstObj client.Object,
	syncedData map[string]string) error {

	existedDstObj, err := deployContext.ClusterAPI.Scheme.New(dstObj.(runtime.Object).GetObjectKind().GroupVersionKind())
	if err != nil {
		return err
	}

	err = deployContext.ClusterAPI.NonCachingClient.Get(
		ctx,
		types.NamespacedName{
			Name:      dstObj.GetName(),
			Namespace: dstObj.GetNamespace()},
		existedDstObj.(client.Object))
	if err == nil {
		// destination object exists, update it if it differs from source object
		srcHasBeenChanged := syncedData[getObjectKey(srcObj)] != srcObj.GetResourceVersion()
		dstHasBeenChanged := syncedData[getObjectKey(dstObj)] != existedDstObj.(client.Object).GetResourceVersion()

		if srcHasBeenChanged || dstHasBeenChanged {
			return doSyncObject(ctx, deployContext, srcObj, dstObj, existedDstObj.(client.Object), syncedData)
		}
	} else if errors.IsNotFound(err) {
		// destination object does not exist, so it will be created
		return doSyncObject(ctx, deployContext, srcObj, dstObj, nil, syncedData)
	} else {
		return err
	}

	return nil
}

// doSyncObject syncs source object to destination object by updating or creating it.
// Returns error if sync failed in a destination namespace.
func doSyncObject(
	ctx context.Context,
	deployContext *chetypes.DeployContext,
	srcObj client.Object,
	dstObj client.Object,
	existedDstObj client.Object,
	syncedData map[string]string) error {

	if existedDstObj == nil {
		if err := deployContext.ClusterAPI.NonCachingClient.Create(ctx, dstObj); err != nil {
			return err
		}
	} else {
		dstObj.SetResourceVersion(existedDstObj.GetResourceVersion())
		if err := deployContext.ClusterAPI.NonCachingClient.Update(ctx, dstObj); err != nil {
			return err
		}
	}

	syncedData[getObjectKey(srcObj)] = srcObj.GetResourceVersion()
	syncedData[getObjectKey(dstObj)] = dstObj.GetResourceVersion()

	log.Info("Object has been synced", "namespace", dstObj.GetNamespace(), "name", dstObj.GetName(), "type", deploy.GetObjectType(dstObj))

	return nil
}

// getSyncedConfig returns ConfigMap with synced objects resource versions.
// Returns error if ConfigMap failed to be retrieved.
func getSyncedConfig(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext) (*corev1.ConfigMap, error) {
	syncedConfig := &corev1.ConfigMap{}
	err := deployContext.ClusterAPI.NonCachingClient.Get(ctx,
		types.NamespacedName{
			Name:      syncedWorkspacesConfig,
			Namespace: targetNs,
		},
		syncedConfig)

	if err != nil {
		if errors.IsNotFound(err) {
			syncedConfig = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      syncedWorkspacesConfig,
					Namespace: targetNs,
					Labels: map[string]string{
						constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
						constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
					},
				},
				Data: map[string]string{},
			}
		} else {
			return nil, err
		}
	} else if syncedConfig.Data == nil {
		syncedConfig.Data = map[string]string{}
	}

	return syncedConfig, nil
}

func getObjectKey(object client.Object) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.%s", deploy.GetObjectType(object), object.GetName(), object.GetNamespace()))
}
