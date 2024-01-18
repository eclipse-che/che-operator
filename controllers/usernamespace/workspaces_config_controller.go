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
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/utils"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	syncedWorkspacesConfig = "sync-workspaces-config"
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
	if req.Name == "" {
		return ctrl.Result{}, nil
	}

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
	if checluster == nil {
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

func (r *WorkspacesConfigReconciler) syncWorkspacesConfig(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext) error {
	syncedConfig, err := getSyncConfig(ctx, targetNs, deployContext)
	if err != nil {
		log.Error(err, "Failed to get workspace sync config", "namespace", targetNs)
		return nil
	}

	defer func() {
		if syncedConfig != nil {
			if syncedConfig.GetResourceVersion() == "" {
				if err := deployContext.ClusterAPI.Client.Create(ctx, syncedConfig); err != nil {
					log.Error(err, "Failed to workspace create sync config", "namespace", targetNs)
				}
			} else {
				if err := deployContext.ClusterAPI.Client.Update(ctx, syncedConfig); err != nil {
					log.Error(err, "Failed to update workspace sync config", "namespace", targetNs)
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

func syncConfigMaps(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext, syncConfig map[string]string) error {
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

		if err := syncObject(ctx, deployContext, &srcObj, dstObj, syncConfig); err != nil {
			log.Error(err, "Failed to sync ConfigMap", "namespace", dstObj.Namespace, "name", dstObj.Name)
		}
	}

	if err := deleteLeftovers(ctx, &corev1.ConfigMap{}, objs, targetNs, deployContext, syncConfig); err != nil {
		log.Error(err, "Failed to delete obsolete ConfigMaps", "namespace", targetNs)
	}

	return nil
}

func syncSecrets(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext, syncConfig map[string]string) error {
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

		if err := syncObject(ctx, deployContext, &srcObj, dstObj, syncConfig); err != nil {
			log.Error(err, "Failed to sync Secret", "namespace", dstObj.Namespace, "name", dstObj.Name)
		}
	}

	if err := deleteLeftovers(ctx, &corev1.Secret{}, objs, targetNs, deployContext, syncConfig); err != nil {
		log.Error(err, "Failed to delete obsolete Secrets", "namespace", targetNs)
	}

	return nil
}

func syncPVC(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext, syncConfig map[string]string) error {
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

		if err := syncObject(ctx, deployContext, &srcObj, dstObj, syncConfig); err != nil {
			log.Error(err, "Failed to sync PersistentVolumeClaim", "namespace", dstObj.Namespace, "name", dstObj.Name)
		}
	}

	if err := deleteLeftovers(ctx, &corev1.PersistentVolumeClaim{}, objs, targetNs, deployContext, syncConfig); err != nil {
		log.Error(err, "Failed to delete obsolete PersistentVolumeClaims", "namespace", targetNs)
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

// deleteLeftovers deletes objects that are not synced with source objects.
func deleteLeftovers(
	ctx context.Context,
	blueprint client.Object,
	objList client.ObjectList,
	targetNs string,
	deployContext *chetypes.DeployContext,
	syncConfig map[string]string) error {

	objs, err := meta.ExtractList(objList)
	if err != nil {
		return err
	}

	actualSyncedObjKeys := make(map[string]bool)
	for _, obj := range objs {
		// compute actual synced objects keys from che namespace
		actualSyncedObjKeys[getObjectKey(obj.(client.Object))] = true
	}

	for syncObjKey, _ := range syncConfig {
		if err := doDeleteLeftovers(
			ctx,
			blueprint,
			actualSyncedObjKeys,
			syncObjKey,
			targetNs,
			deployContext,
			syncConfig); err != nil {
			log.Error(err, "Failed to delete obsolete object", "namespace", targetNs, "kind", deploy.GetObjectType(blueprint), "name", getObjectNameFromKey(syncObjKey))
		}
	}

	return nil
}

// doDeleteLeftovers deletes objects that are not synced with source objects.
// Returns error if delete failed in a destination namespace.
func doDeleteLeftovers(
	ctx context.Context,
	blueprint client.Object,
	actualSyncedObjKeys map[string]bool,
	syncObjKey string,
	targetNs string,
	deployContext *chetypes.DeployContext,
	syncConfig map[string]string,
) error {
	isObjectOfGivenType := strings.HasPrefix(syncObjKey, deploy.GetObjectType(blueprint))
	isObjectFromCheNamespace := strings.HasSuffix(syncObjKey, deployContext.CheCluster.GetNamespace())
	isNotSyncedInTargetNs := !actualSyncedObjKeys[syncObjKey]

	if isObjectOfGivenType && isObjectFromCheNamespace && isNotSyncedInTargetNs {
		// then delete object from target namespace if it is not synced with source object
		objName := getObjectNameFromKey(syncObjKey)
		if err := deploy.DeleteIgnoreIfNotFound(
			ctx,
			deployContext.ClusterAPI.NonCachingClient,
			types.NamespacedName{
				Name:      objName,
				Namespace: targetNs,
			},
			blueprint); err != nil {
			return err
		}

		delete(syncConfig, syncObjKey)
		delete(syncConfig, computeObjectKey(deploy.GetObjectType(blueprint), objName, targetNs))
	}

	return nil
}

// syncObject syncs source object to destination object if they differ.
// Returns error if sync failed in a destination namespace.
func syncObject(
	ctx context.Context,
	deployContext *chetypes.DeployContext,
	srcObj client.Object,
	dstObj client.Object,
	syncConfig map[string]string) error {

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
		srcHasBeenChanged := syncConfig[getObjectKey(srcObj)] != srcObj.GetResourceVersion()
		dstHasBeenChanged := syncConfig[getObjectKey(dstObj)] != existedDstObj.(client.Object).GetResourceVersion()

		if srcHasBeenChanged || dstHasBeenChanged {
			return doSyncObject(ctx, deployContext, srcObj, dstObj, existedDstObj.(client.Object), syncConfig)
		}
	} else if errors.IsNotFound(err) {
		// destination object does not exist, so it will be created
		return doSyncObject(ctx, deployContext, srcObj, dstObj, nil, syncConfig)
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
	syncConfig map[string]string) error {

	if existedDstObj == nil {
		if err := deployContext.ClusterAPI.Client.Create(ctx, dstObj); err != nil {
			return err
		}
	} else {
		dstObj.SetResourceVersion(existedDstObj.GetResourceVersion())
		if err := deployContext.ClusterAPI.Client.Update(ctx, dstObj); err != nil {
			return err
		}
	}

	syncConfig[getObjectKey(srcObj)] = srcObj.GetResourceVersion()
	syncConfig[getObjectKey(dstObj)] = dstObj.GetResourceVersion()

	log.Info("Object synced", "namespace", dstObj.GetNamespace(), "kind", deploy.GetObjectType(dstObj), "name", dstObj.GetName())

	return nil
}

// getSyncConfig returns ConfigMap with synced objects resource versions.
// Returns error if ConfigMap failed to be retrieved.
func getSyncConfig(ctx context.Context, targetNs string, deployContext *chetypes.DeployContext) (*corev1.ConfigMap, error) {
	syncedConfig := &corev1.ConfigMap{}
	err := deployContext.ClusterAPI.Client.Get(ctx,
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
						constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
						constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
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
	return fmt.Sprintf("%s.%s.%s", deploy.GetObjectType(object), object.GetName(), object.GetNamespace())
}

func computeObjectKey(kind string, name string, namespace string) string {
	return fmt.Sprintf("%s.%s.%s", kind, name, namespace)
}

func getObjectNameFromKey(key string) string {
	splits := strings.Split(key, ".")
	return splits[len(splits)-2]
}

func isLabeledAsWorkspacesConfig(obj metav1.Object) bool {
	return obj.GetLabels()[constants.KubernetesComponentLabelKey] == constants.WorkspacesConfig
}

func addDefaultLabels(labels map[string]string) {
	utils.AddMap(labels, deploy.GetLabels(constants.WorkspacesConfig))
}
