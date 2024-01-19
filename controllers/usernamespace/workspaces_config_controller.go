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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type WorkspacesConfigReconciler struct {
	scheme          *runtime.Scheme
	client          client.Client
	nonCachedClient client.Client
	namespaceCache  *namespaceCache
}

var (
	log = ctrl.Log.WithName("workspaces-config")

	workspacesConfigSelector = labels.SelectorFromSet(map[string]string{
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
	})

	v1SecretGKV    = corev1.SchemeGroupVersion.WithKind("Secret")
	v1ConfigMapGKV = corev1.SchemeGroupVersion.WithKind("ConfigMap")
	v1PvcGKV       = corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
)

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

	checluster, err := deploy.FindCheClusterCRInNamespace(r.client, "")
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
							cheCluster, _ := deploy.FindCheClusterCRInNamespace(r.client, o.GetNamespace())
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

	if err := syncObjects(ctx, targetNs, deployContext, newConfigMap, v1ConfigMapGKV, &corev1.ConfigMapList{}, syncedConfig.Data); err != nil {
		return err
	}

	if err := syncObjects(ctx, targetNs, deployContext, newSecret, v1SecretGKV, &corev1.SecretList{}, syncedConfig.Data); err != nil {
		return err
	}

	if err := syncObjects(ctx, targetNs, deployContext, newPVC, v1PvcGKV, &corev1.PersistentVolumeClaimList{}, syncedConfig.Data); err != nil {
		return err
	}

	return nil
}

func newConfigMap(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.ConfigMap).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels: getWorkspaceConfigObjectLabels(
			src.GetLabels(),
			map[string]string{
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
			},
		),
	}

	return dst.(client.Object)
}

func newSecret(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.Secret).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels: getWorkspaceConfigObjectLabels(
			src.GetLabels(),
			map[string]string{
				dwconstants.DevWorkspaceWatchSecretLabel: "true",
				dwconstants.DevWorkspaceMountLabel:       "true",
			},
		),
	}

	return dst.(client.Object)
}

func newPVC(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.PersistentVolumeClaim).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels:      src.GetLabels(),
	}
	dst.(*corev1.PersistentVolumeClaim).Status = corev1.PersistentVolumeClaimStatus{}

	return dst.(client.Object)
}

// syncObjects syncs objects from che namespace to target namespace.
func syncObjects(
	ctx context.Context,
	targetNs string,
	deployContext *chetypes.DeployContext,
	newObjectFunc func(srcObject client.Object) client.Object,
	gkv schema.GroupVersionKind,
	srcObjsList client.ObjectList,
	syncConfig map[string]string) error {

	if err := readSrcObjsList(ctx, deployContext, srcObjsList); err != nil {
		return err
	}

	srcObjs, err := meta.ExtractList(srcObjsList)
	if err != nil {
		return err
	}

	newObjVersionAndKind := fmt.Sprintf("%s.%s", gkv.Version, gkv.Kind)
	for _, srcObj := range srcObjs {
		newObj := newObjectFunc(srcObj.(client.Object))
		newObj.SetNamespace(targetNs)

		if err := syncObjectToNamespace(ctx, deployContext, srcObj.(client.Object), newObj, syncConfig); err != nil {
			log.Error(err, "Failed to sync object",
				"namespace", targetNs,
				"kind", newObjVersionAndKind,
				"name", newObj.GetName())
			return err
		}
	}

	actualSyncedSrcObjKeys := make(map[string]bool)
	for _, srcObj := range srcObjs {
		// compute actual synced objects keys from che namespace
		actualSyncedSrcObjKeys[getObjectKey(srcObj.(client.Object))] = true
	}

	for syncObjKey, _ := range syncConfig {
		if err := deleteObsoleteObjectFromNamespace(
			ctx,
			gkv,
			actualSyncedSrcObjKeys,
			syncObjKey,
			targetNs,
			deployContext,
			syncConfig); err != nil {
			log.Error(err, "Failed to delete obsolete object",
				"namespace", targetNs,
				"kind", newObjVersionAndKind,
				"name", getObjectNameFromKey(syncObjKey))
			return err
		}
	}

	return nil
}

func readSrcObjsList(ctx context.Context, deployContext *chetypes.DeployContext, objList client.ObjectList) error {
	return deployContext.ClusterAPI.Client.List(
		ctx,
		objList,
		&client.ListOptions{
			Namespace:     deployContext.CheCluster.Namespace,
			LabelSelector: workspacesConfigSelector,
		})
}

// deleteObsoleteObjectFromNamespace deletes objects that are not synced with source objects.
// Returns error if delete failed in a destination namespace.
func deleteObsoleteObjectFromNamespace(
	ctx context.Context,
	gkv schema.GroupVersionKind,
	actualSyncedSrcObjKeys map[string]bool,
	syncObjKey string,
	targetNs string,
	deployContext *chetypes.DeployContext,
	syncConfig map[string]string,
) error {
	isObjectOfGivenKind := getObjectGVKFromKey(syncObjKey) == gkv2KeyItem(gkv)
	isObjectFromCheNamespace := getObjectNamespaceFromKey(syncObjKey) == deployContext.CheCluster.GetNamespace()
	isNotSyncedInTargetNs := !actualSyncedSrcObjKeys[syncObjKey]

	if isObjectOfGivenKind && isObjectFromCheNamespace && isNotSyncedInTargetNs {
		blueprint, _ := deployContext.ClusterAPI.Scheme.New(gkv)

		// then delete object from target namespace if it is not synced with source object
		if err := deploy.DeleteIgnoreIfNotFound(
			ctx,
			deployContext.ClusterAPI.Client,
			types.NamespacedName{
				Name:      getObjectNameFromKey(syncObjKey),
				Namespace: targetNs,
			},
			blueprint.(client.Object)); err != nil {
			return err
		}

		delete(syncConfig, syncObjKey)
		delete(syncConfig, computeObjectKey(gkv, getObjectNameFromKey(syncObjKey), targetNs))
	}

	return nil
}

// syncObjectToNamespace syncs source object to destination object if they differ.
// Returns error if sync failed in a destination namespace.
func syncObjectToNamespace(
	ctx context.Context,
	deployContext *chetypes.DeployContext,
	srcObj client.Object,
	newObj client.Object,
	syncConfig map[string]string) error {

	gkv := srcObj.GetObjectKind().GroupVersionKind()

	existedDstObj, err := deployContext.ClusterAPI.Scheme.New(gkv)
	if err != nil {
		return err
	}

	err = deployContext.ClusterAPI.Client.Get(
		ctx,
		types.NamespacedName{
			Name:      newObj.GetName(),
			Namespace: newObj.GetNamespace()},
		existedDstObj.(client.Object))
	if err == nil {
		// destination object exists, update it if it differs from source object
		srcHasBeenChanged := syncConfig[getObjectKey(srcObj)] != srcObj.GetResourceVersion()
		dstHasBeenChanged := syncConfig[computeObjectKey(gkv, newObj.GetName(), newObj.GetNamespace())] != existedDstObj.(client.Object).GetResourceVersion()

		if srcHasBeenChanged || dstHasBeenChanged {
			return doSyncObjectToNamespace(ctx, gkv, deployContext, srcObj, newObj, existedDstObj.(client.Object).GetResourceVersion(), syncConfig)
		}
	} else if errors.IsNotFound(err) {
		// destination object does not exist, so it will be created
		return doSyncObjectToNamespace(ctx, gkv, deployContext, srcObj, newObj, "", syncConfig)
	} else {
		return err
	}

	return nil
}

// doSyncObjectToNamespace syncs source object to destination object by updating or creating it.
// Returns error if sync failed in a destination namespace.
func doSyncObjectToNamespace(
	ctx context.Context,
	gkv schema.GroupVersionKind,
	deployContext *chetypes.DeployContext,
	srcObj client.Object,
	newObj client.Object,
	existedDstObjResourceVersion string,
	syncConfig map[string]string) error {

	if existedDstObjResourceVersion == "" {
		if err := deployContext.ClusterAPI.Client.Create(ctx, newObj); err != nil {
			return err
		}
	} else {
		if isUpdateUsingDeleteCreate(gkv.Kind) {
			blueprint, _ := deployContext.ClusterAPI.Scheme.New(gkv)
			if err := deploy.DeleteIgnoreIfNotFound(
				ctx,
				deployContext.ClusterAPI.Client,
				types.NamespacedName{
					Name:      newObj.GetName(),
					Namespace: newObj.GetNamespace(),
				},
				blueprint.(client.Object)); err != nil {
				return err
			}

			if err := deployContext.ClusterAPI.Client.Create(ctx, newObj); err != nil {
				return err
			}
		} else {
			newObj.SetResourceVersion(existedDstObjResourceVersion)
			if err := deployContext.ClusterAPI.Client.Update(ctx, newObj); err != nil {
				return err
			}
		}
	}

	syncConfig[getObjectKey(srcObj)] = srcObj.GetResourceVersion()
	syncConfig[computeObjectKey(gkv, newObj.GetName(), newObj.GetNamespace())] = newObj.GetResourceVersion()

	log.Info("Object synced",
		"namespace", newObj.GetNamespace(),
		"kind", fmt.Sprintf("%s.%s", gkv.Version, gkv.Kind),
		"name", newObj.GetName())
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

func isUpdateUsingDeleteCreate(kind string) bool {
	return "PersistentVolumeClaim" == kind
}

func computeObjectKey(gvk schema.GroupVersionKind, name string, namespace string) string {
	return fmt.Sprintf("%s.%s.%s", gkv2KeyItem(gvk), name, namespace)
}

func getObjectKey(object client.Object) string {
	return computeObjectKey(object.GetObjectKind().GroupVersionKind(), object.GetName(), object.GetNamespace())
}

func gkv2KeyItem(gvk schema.GroupVersionKind) string {
	if gvk.Group == "" {
		return fmt.Sprintf("%s_%s", gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("%s_%s_%s", gvk.Group, gvk.Version, gvk.Kind)
}

func getObjectGVKFromKey(key string) string {
	splits := strings.Split(key, ".")
	return splits[0]
}

func getObjectNameFromKey(key string) string {
	splits := strings.Split(key, ".")
	return splits[1]
}

func getObjectNamespaceFromKey(key string) string {
	splits := strings.Split(key, ".")
	return splits[2]
}

func isLabeledAsWorkspacesConfig(obj metav1.Object) bool {
	return obj.GetLabels()[constants.KubernetesComponentLabelKey] == constants.WorkspacesConfig &&
		obj.GetLabels()[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg
}

func getWorkspaceConfigObjectLabels(srcLabels map[string]string, newLabels map[string]string) map[string]string {
	dstLabels := utils.CloneMap(srcLabels)
	for key, value := range newLabels {
		dstLabels[key] = value
	}

	// default labels
	for key, value := range deploy.GetLabels(constants.WorkspacesConfig) {
		dstLabels[key] = value
	}

	return dstLabels
}
