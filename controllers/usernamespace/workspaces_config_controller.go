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

// Interface for syncing workspace config objects.
type workspaceConfigSyncer interface {
	gkv() schema.GroupVersionKind
	isExistedObjChanged(newObj client.Object, existedObj client.Object) bool
	hasReadOnlySpec() bool
	getObjectList() client.ObjectList
	newObjectFrom(src client.Object) client.Object
}

type syncContext struct {
	dstNamespace string
	srcNamespace string
	ctx          context.Context
	syncer       workspaceConfigSyncer
	syncConfig   map[string]string
}

var (
	log = ctrl.Log.WithName("workspaces-config")

	workspacesConfigLabels = map[string]string{
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
	}
	workspacesConfigSelector = labels.SelectorFromSet(workspacesConfigLabels)
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

	if err = r.syncWorkspacesConfig(ctx, req.Name); err != nil {
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

func (r *WorkspacesConfigReconciler) syncWorkspacesConfig(ctx context.Context, targetNs string) error {
	checluster, err := deploy.FindCheClusterCRInNamespace(r.client, "")
	if checluster == nil {
		return nil
	}

	syncedConfig, err := r.getSyncConfig(ctx, targetNs)
	if err != nil {
		log.Error(err, "Failed to get workspace sync config", "namespace", targetNs)
		return nil
	}

	defer func() {
		if syncedConfig != nil {
			if syncedConfig.GetResourceVersion() == "" {
				if err := r.client.Create(ctx, syncedConfig); err != nil {
					log.Error(err, "Failed to workspace create sync config", "namespace", targetNs)
				}
			} else {
				if err := r.client.Update(ctx, syncedConfig); err != nil {
					log.Error(err, "Failed to update workspace sync config", "namespace", targetNs)
				}
			}
		}
	}()

	if err := r.syncObjects(
		&syncContext{
			dstNamespace: targetNs,
			srcNamespace: checluster.GetNamespace(),
			syncer:       newConfigMapSyncer(),
			syncConfig:   syncedConfig.Data,
			ctx:          ctx,
		}); err != nil {
		return err
	}

	if err := r.syncObjects(
		&syncContext{
			dstNamespace: targetNs,
			srcNamespace: checluster.GetNamespace(),
			syncer:       newSecretSyncer(),
			syncConfig:   syncedConfig.Data,
			ctx:          ctx,
		}); err != nil {
		return err
	}

	if err := r.syncObjects(
		&syncContext{
			dstNamespace: targetNs,
			srcNamespace: checluster.GetNamespace(),
			syncer:       newPvcSyncer(),
			syncConfig:   syncedConfig.Data,
			ctx:          ctx,
		}); err != nil {
		return err
	}

	return nil
}

// syncObjects syncs objects from che namespace to target namespace.
func (r *WorkspacesConfigReconciler) syncObjects(syncContext *syncContext) error {
	srcObjsList := syncContext.syncer.getObjectList()
	if err := r.readSrcObjsList(syncContext.ctx, syncContext.srcNamespace, srcObjsList); err != nil {
		return err
	}

	srcObjs, err := meta.ExtractList(srcObjsList)
	if err != nil {
		return err
	}

	for _, srcObj := range srcObjs {
		newObj := syncContext.syncer.newObjectFrom(srcObj.(client.Object))
		newObj.SetNamespace(syncContext.dstNamespace)

		if err := r.syncObjectToNamespace(syncContext, srcObj.(client.Object), newObj); err != nil {
			log.Error(err, "Failed to sync object",
				"namespace", syncContext.dstNamespace,
				"kind", gvk2String(syncContext.syncer.gkv()),
				"name", newObj.GetName())
			return err
		}
	}

	actualSyncedSrcObjKeys := make(map[string]bool)
	for _, srcObj := range srcObjs {
		// compute actual synced objects keys from che namespace
		actualSyncedSrcObjKeys[getKey(srcObj.(client.Object))] = true
	}

	for syncObjKey, _ := range syncContext.syncConfig {
		if err := r.deleteObsoleteObjectFromNamespace(syncContext, actualSyncedSrcObjKeys, syncObjKey); err != nil {
			log.Error(err, "Failed to delete obsolete object",
				"namespace", syncContext.dstNamespace,
				"kind", gvk2String(syncContext.syncer.gkv()),
				"name", getNameElement(syncObjKey))
			return err
		}
	}

	return nil
}

// deleteObsoleteObjectFromNamespace deletes objects that are not synced with source objects.
// Returns error if delete failed in a destination namespace.
func (r *WorkspacesConfigReconciler) deleteObsoleteObjectFromNamespace(
	syncContext *syncContext,
	actualSyncedSrcObjKeys map[string]bool,
	syncObjKey string,
) error {
	isObjectOfGivenKind := getGVKElement(syncObjKey) == gvk2Element(syncContext.syncer.gkv())
	isObjectFromSrcNamespace := getNamespaceElement(syncObjKey) == syncContext.srcNamespace
	isNotSyncedInTargetNs := !actualSyncedSrcObjKeys[syncObjKey]

	if isObjectOfGivenKind && isObjectFromSrcNamespace && isNotSyncedInTargetNs {
		blueprint, err := r.scheme.New(syncContext.syncer.gkv())
		if err != nil {
			return err
		}

		// then delete object from target namespace if it is not synced with source object
		if err := deploy.DeleteIgnoreIfNotFound(
			syncContext.ctx,
			r.client,
			types.NamespacedName{
				Name:      getNameElement(syncObjKey),
				Namespace: syncContext.dstNamespace,
			},
			blueprint.(client.Object)); err != nil {
			return err
		}

		delete(syncContext.syncConfig, syncObjKey)
		delete(syncContext.syncConfig,
			buildKey(
				syncContext.syncer.gkv(),
				getNameElement(syncObjKey),
				syncContext.dstNamespace),
		)
	}

	return nil
}

// syncObjectToNamespace syncs source object to destination object if they differ.
// Returns error if sync failed in a destination namespace.
func (r *WorkspacesConfigReconciler) syncObjectToNamespace(
	syncContext *syncContext,
	srcObj client.Object,
	newObj client.Object) error {

	existedDstObj, err := r.scheme.New(syncContext.syncer.gkv())
	if err != nil {
		return err
	}

	err = r.client.Get(
		syncContext.ctx,
		types.NamespacedName{
			Name:      newObj.GetName(),
			Namespace: newObj.GetNamespace()},
		existedDstObj.(client.Object))
	if err == nil {
		// destination object exists, update it if it differs from source object
		srcHasBeenChanged := syncContext.syncConfig[getKey(srcObj)] != srcObj.GetResourceVersion()
		dstHasBeenChanged := syncContext.syncConfig[getKey(existedDstObj.(client.Object))] != existedDstObj.(client.Object).GetResourceVersion()

		if srcHasBeenChanged || dstHasBeenChanged {
			return r.doSyncObjectToNamespace(syncContext, srcObj, newObj, existedDstObj.(client.Object))
		}
	} else if errors.IsNotFound(err) {
		// destination object does not exist, so it will be created
		return r.doSyncObjectToNamespace(syncContext, srcObj, newObj, nil)
	} else {
		return err
	}

	return nil
}

// doSyncObjectToNamespace syncs source object to destination object by updating or creating it.
// Returns error if sync failed in a destination namespace.
func (r *WorkspacesConfigReconciler) doSyncObjectToNamespace(
	syncContext *syncContext,
	srcObj client.Object,
	newObj client.Object,
	existedObj client.Object) error {

	if existedObj == nil {
		if err := r.client.Create(syncContext.ctx, newObj); err != nil {
			return err
		}

		syncContext.syncConfig[getKey(srcObj)] = srcObj.GetResourceVersion()
		syncContext.syncConfig[buildKey(
			syncContext.syncer.gkv(),
			newObj.GetName(),
			newObj.GetNamespace())] = newObj.GetResourceVersion()

		log.Info("Object created",
			"namespace", newObj.GetNamespace(),
			"kind", gvk2String(syncContext.syncer.gkv()),
			"name", newObj.GetName())
		return nil
	} else {
		if syncContext.syncer.hasReadOnlySpec() {
			// skip updating objects with readonly spec
			// admin has to re-create them to update
			// just update resource versions
			syncContext.syncConfig[getKey(srcObj)] = srcObj.GetResourceVersion()
			syncContext.syncConfig[getKey(existedObj)] = existedObj.GetResourceVersion()

			log.Info("Object skipped since has readonly spec, re-create it to update",
				"namespace", newObj.GetNamespace(),
				"kind", gvk2String(syncContext.syncer.gkv()),
				"name", newObj.GetName())
			return nil
		} else {
			if syncContext.syncer.isExistedObjChanged(newObj, existedObj) {
				// preserve labels and annotations from existed object
				newObj.SetLabels(preserveExistedMapValues(newObj.GetLabels(), existedObj.GetLabels()))
				newObj.SetAnnotations(preserveExistedMapValues(newObj.GetAnnotations(), existedObj.GetAnnotations()))

				// set the correct resource version to update object
				newObj.SetResourceVersion(existedObj.GetResourceVersion())
				if err := r.client.Update(syncContext.ctx, newObj); err != nil {
					return err
				}

				syncContext.syncConfig[getKey(srcObj)] = srcObj.GetResourceVersion()
				syncContext.syncConfig[getKey(existedObj)] = newObj.GetResourceVersion()

				log.Info("Object updated",
					"namespace", newObj.GetNamespace(),
					"kind", gvk2String(syncContext.syncer.gkv()),
					"name", newObj.GetName())
				return nil
			} else {
				// nothing to update objects are equal
				// just update resource versions
				syncContext.syncConfig[getKey(srcObj)] = srcObj.GetResourceVersion()
				syncContext.syncConfig[getKey(existedObj)] = existedObj.GetResourceVersion()
				return nil
			}
		}
	}
}

// getSyncConfig returns ConfigMap with synced objects resource versions.
// Returns error if ConfigMap failed to be retrieved.
func (r *WorkspacesConfigReconciler) getSyncConfig(ctx context.Context, targetNs string) (*corev1.ConfigMap, error) {
	syncedConfig := &corev1.ConfigMap{}
	err := r.client.Get(
		ctx,
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
					Labels:    workspacesConfigLabels,
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

func (r *WorkspacesConfigReconciler) readSrcObjsList(ctx context.Context, srcNamespace string, objList client.ObjectList) error {
	return r.client.List(
		ctx,
		objList,
		&client.ListOptions{
			Namespace:     srcNamespace,
			LabelSelector: workspacesConfigSelector,
		})
}

func getKey(object client.Object) string {
	return buildKey(object.GetObjectKind().GroupVersionKind(), object.GetName(), object.GetNamespace())
}

func buildKey(gvk schema.GroupVersionKind, name string, namespace string) string {
	return fmt.Sprintf("%s.%s.%s", gvk2Element(gvk), name, namespace)
}

func gvk2Element(gvk schema.GroupVersionKind) string {
	if gvk.Group == "" {
		return fmt.Sprintf("%s_%s", gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("%s_%s_%s", gvk.Group, gvk.Version, gvk.Kind)
}

func gvk2String(gkv schema.GroupVersionKind) string {
	return fmt.Sprintf("%s.%s", gkv.Version, gkv.Kind)
}

func getGVKElement(key string) string {
	splits := strings.Split(key, ".")
	return splits[0]
}

func getNameElement(key string) string {
	splits := strings.Split(key, ".")
	return splits[1]
}

func getNamespaceElement(key string) string {
	splits := strings.Split(key, ".")
	return splits[2]
}

func isLabeledAsWorkspacesConfig(obj metav1.Object) bool {
	return obj.GetLabels()[constants.KubernetesComponentLabelKey] == constants.WorkspacesConfig &&
		obj.GetLabels()[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg
}

func mergeWorkspaceConfigObjectLabels(srcLabels map[string]string, additionalLabels map[string]string) map[string]string {
	newLabels := utils.CloneMap(srcLabels)
	for key, value := range additionalLabels {
		newLabels[key] = value
	}

	// default labels
	for key, value := range deploy.GetLabels(constants.WorkspacesConfig) {
		newLabels[key] = value
	}

	return newLabels
}

func preserveExistedMapValues(newObjMap map[string]string, existedObjMap map[string]string) map[string]string {
	preservedMap := utils.CloneMap(newObjMap)
	for key, value := range existedObjMap {
		if _, ok := preservedMap[key]; !ok {
			preservedMap[key] = value
		}
	}
	return preservedMap
}
