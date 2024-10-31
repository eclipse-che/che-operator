//
// Copyright (c) 2019-2024 Red Hat, Inc.
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

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	templatev1 "github.com/openshift/api/template/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	scheme         *runtime.Scheme
	client         client.Client
	namespaceCache *namespaceCache
}

type Object2Sync interface {
	getGKV() schema.GroupVersionKind
	hasROSpec() bool
	getSrcObject() client.Object
	getSrcObjectVersion() string
	newDstObject() client.Object
	isDiff(obj client.Object) bool
}

type syncContext struct {
	dstNamespace string
	srcNamespace string
	ctx          context.Context
	object2Sync  Object2Sync
	syncConfig   map[string]string
}

var (
	logger                  = ctrl.Log.WithName("workspaces-config")
	wsConfigComponentLabels = map[string]string{
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
	}
	wsConfigComponentSelector = labels.SelectorFromSet(wsConfigComponentLabels)
)

func NewWorkspacesConfigReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	namespaceCache *namespaceCache) *WorkspacesConfigReconciler {

	return &WorkspacesConfigReconciler{
		scheme:         scheme,
		client:         client,
		namespaceCache: namespaceCache,
	}
}

func (r *WorkspacesConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, r.watchRules(ctx, true, true)).
		Watches(&source.Kind{Type: &corev1.Secret{}}, r.watchRules(ctx, true, true)).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, r.watchRules(ctx, true, true)).
		Watches(&source.Kind{Type: &corev1.ResourceQuota{}}, r.watchRules(ctx, false, true)).
		Watches(&source.Kind{Type: &corev1.LimitRange{}}, r.watchRules(ctx, false, true)).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, r.watchRules(ctx, false, true)).
		Watches(&source.Kind{Type: &rbacv1.Role{}}, r.watchRules(ctx, false, true)).
		Watches(&source.Kind{Type: &rbacv1.RoleBinding{}}, r.watchRules(ctx, false, true))

	if infrastructure.IsOpenShift() {
		bld.Watches(&source.Kind{Type: &templatev1.Template{}}, r.watchRules(ctx, true, false))
	}

	return bld.Complete(r)
}

func (r *WorkspacesConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name == "" {
		return ctrl.Result{}, nil
	}

	checluster, err := deploy.FindCheClusterCRInNamespace(r.client, "")
	if checluster == nil {
		// There is no CheCluster CR, the source namespace is unknown
		return ctrl.Result{}, nil
	}

	info, err := r.namespaceCache.ExamineNamespace(ctx, req.Name)
	if err != nil {
		logger.Error(err, "Failed to examine namespace", "namespace", req.Name)
		return ctrl.Result{}, err
	}

	if info == nil || !info.IsWorkspaceNamespace {
		// namespace is not a workspace namespace, nothing to do
		return ctrl.Result{}, nil
	}

	if info.Username == "" {
		logger.Info("Username is not set for the namespace", "namespace", req.Name)
		return ctrl.Result{}, nil
	}

	if err = r.syncWorkspace(ctx, checluster.Namespace, req.Name); err != nil {
		logger.Error(err, "Failed to sync workspace configs", "namespace", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Establish watch rules for object.
// cheNamespaceRule - if true, then watch changes in che namespace (source namespace)
// userNamespaceRule - if true, then watch changes in user namespaces (destination namespaces)
func (r *WorkspacesConfigReconciler) watchRules(
	ctx context.Context,
	cheNamespaceRule bool,
	userNamespaceRule bool,
) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(obj client.Object) []reconcile.Request {
			var eventRules []eventRule

			if cheNamespaceRule {
				eventRules = append(eventRules,
					eventRule{
						// reconcile rule when workspace config is modified in a che namespace
						// to update the config in all users` namespaces
						check: func(o metav1.Object) bool {
							cheCluster, _ := deploy.FindCheClusterCRInNamespace(r.client, o.GetNamespace())
							return hasWSConfigComponentLabels(o) && cheCluster != nil
						},
						namespaces: func(o metav1.Object) []string { return r.namespaceCache.GetAllKnownNamespaces() },
					},
				)
			}

			if userNamespaceRule {
				eventRules = append(eventRules,
					eventRule{
						// reconcile rule when workspace config is modified in a user namespace
						// to revert the config
						check: func(o metav1.Object) bool {
							workspaceInfo, _ := r.namespaceCache.GetNamespaceInfo(ctx, o.GetNamespace())
							return hasWSConfigComponentLabels(o) &&
								o.GetName() != syncedWorkspacesConfig &&
								workspaceInfo != nil &&
								workspaceInfo.IsWorkspaceNamespace
						},
						namespaces: func(o metav1.Object) []string { return []string{o.GetNamespace()} },
					},
				)
			}

			return asReconcileRequestsForNamespaces(obj, eventRules)
		})
}

// syncWorkspace sync user namespace.
// Iterates over all objects in the source namespace labeled as `app.kubernetes.io/component=workspaces-config`
// and syncs them to the target user namespace.
func (r *WorkspacesConfigReconciler) syncWorkspace(
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
) error {
	syncConfig, err := r.getSyncConfig(ctx, dstNamespace)
	if err != nil {
		return err
	}

	defer func() {
		// Update sync config in the end of the reconciliation
		// despite the result of the reconciliation
		if syncConfig != nil {
			if syncConfig.GetResourceVersion() == "" {
				if err := r.client.Create(ctx, syncConfig); err != nil {
					logger.Error(err, "Failed to workspace create sync config", "namespace", dstNamespace)
				}
			} else {
				if err := r.client.Update(ctx, syncConfig); err != nil {
					logger.Error(err, "Failed to update workspace sync config", "namespace", dstNamespace)
				}
			}
		}
	}()

	// Contains keys of objects that are synced with source objects
	syncedSrcObjKeys := make(map[string]bool)

	if infrastructure.IsOpenShift() {
		if err = r.syncTemplates(
			ctx,
			srcNamespace,
			dstNamespace,
			syncConfig.Data,
			syncedSrcObjKeys,
		); err != nil {
			return err
		}
	}

	if err = r.syncConfigMaps(
		ctx,
		srcNamespace,
		dstNamespace,
		syncConfig.Data,
		syncedSrcObjKeys,
	); err != nil {
		return err
	}

	if err = r.syncSecretes(
		ctx,
		srcNamespace,
		dstNamespace,
		syncConfig.Data,
		syncedSrcObjKeys,
	); err != nil {
		return err
	}

	if err = r.syncPVCs(
		ctx,
		srcNamespace,
		dstNamespace,
		syncConfig.Data,
		syncedSrcObjKeys,
	); err != nil {
		return err
	}

	// Iterates over sync config and deletes obsolete objects, if so.
	// It means that object key presents in sync config, but the object is not synced with source object.
	for objKey, _ := range syncConfig.Data {
		if err := r.deleteIfObjectIsObsolete(
			objKey,
			ctx,
			srcNamespace,
			dstNamespace,
			syncConfig.Data,
			syncedSrcObjKeys); err != nil {

			logger.Error(err, "Failed to delete obsolete object", "namespace", dstNamespace,
				"kind", gvk2PrintString(item2gkv(getGkvItem(objKey))),
				"name", getNameItem(objKey))
			return err
		}
	}

	return nil
}

// syncConfigMaps syncs all ConfigMaps labeled as `app.kubernetes.io/component=workspaces-config`
// from source namespace to a target user namespace.
func (r *WorkspacesConfigReconciler) syncConfigMaps(
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
	syncConfig map[string]string,
	syncedSrcObjKeys map[string]bool) error {

	cmList := &corev1.ConfigMapList{}
	opts := &client.ListOptions{
		Namespace:     srcNamespace,
		LabelSelector: wsConfigComponentSelector,
	}
	if err := r.client.List(ctx, cmList, opts); err != nil {
		return err
	}

	for _, cm := range cmList.Items {
		if err := r.syncObject(
			&syncContext{
				dstNamespace: dstNamespace,
				srcNamespace: srcNamespace,
				object2Sync:  newCM2Sync(&cm),
				syncConfig:   syncConfig,
				ctx:          ctx,
			}); err != nil {
			return err
		}

		srcObjKey := buildKey(cm.GroupVersionKind(), cm.GetName(), srcNamespace)
		syncedSrcObjKeys[srcObjKey] = true
	}

	return nil
}

// syncSecretes syncs all Secrets labeled as `app.kubernetes.io/component=workspaces-config`
// from source namespace to a target user namespace.
func (r *WorkspacesConfigReconciler) syncSecretes(
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
	syncConfig map[string]string,
	syncedSrcObjKeys map[string]bool) error {

	secretList := &corev1.SecretList{}
	opts := &client.ListOptions{
		Namespace:     srcNamespace,
		LabelSelector: wsConfigComponentSelector,
	}
	if err := r.client.List(ctx, secretList, opts); err != nil {
		return err
	}

	for _, secret := range secretList.Items {
		if err := r.syncObject(
			&syncContext{
				dstNamespace: dstNamespace,
				srcNamespace: srcNamespace,
				object2Sync:  newSecret2Sync(&secret),
				syncConfig:   syncConfig,
				ctx:          ctx,
			}); err != nil {
			return err
		}

		srcObjKey := buildKey(secret.GroupVersionKind(), secret.GetName(), srcNamespace)
		syncedSrcObjKeys[srcObjKey] = true
	}

	return nil
}

// syncPVCs syncs all PVCs labeled as `app.kubernetes.io/component=workspaces-config`
// from source namespace to a target user namespace.
func (r *WorkspacesConfigReconciler) syncPVCs(
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
	syncConfig map[string]string,
	syncedSrcObjKeys map[string]bool) error {

	pvcList := &corev1.PersistentVolumeClaimList{}
	opts := &client.ListOptions{
		Namespace:     srcNamespace,
		LabelSelector: wsConfigComponentSelector,
	}
	if err := r.client.List(ctx, pvcList, opts); err != nil {
		return err
	}

	for _, pvc := range pvcList.Items {
		if err := r.syncObject(
			&syncContext{
				dstNamespace: dstNamespace,
				srcNamespace: srcNamespace,
				object2Sync:  newPvc2Sync(&pvc),
				syncConfig:   syncConfig,
				ctx:          ctx,
			}); err != nil {
			return err
		}

		srcObjKey := buildKey(pvc.GroupVersionKind(), pvc.GetName(), srcNamespace)
		syncedSrcObjKeys[srcObjKey] = true
	}

	return nil
}

// syncTemplates syncs all objects declared in the template labeled as `app.kubernetes.io/component=workspaces-config`
// from source namespace to a target user namespace.
func (r *WorkspacesConfigReconciler) syncTemplates(
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
	syncConfig map[string]string,
	syncedSrcObjKeys map[string]bool) error {

	templates := &templatev1.TemplateList{}
	opts := &client.ListOptions{
		Namespace:     srcNamespace,
		LabelSelector: wsConfigComponentSelector,
	}
	if err := r.client.List(ctx, templates, opts); err != nil {
		return err
	}

	nsInfo, err := r.namespaceCache.GetNamespaceInfo(ctx, dstNamespace)
	if err != nil {
		return nil
	}

	for _, template := range templates.Items {
		for _, object := range template.Objects {
			object2Sync, err := newUnstructured2Sync(object.Raw, nsInfo.Username, dstNamespace)
			if err != nil {
				return err
			}

			if err = r.syncObject(
				&syncContext{
					dstNamespace: dstNamespace,
					srcNamespace: srcNamespace,
					object2Sync:  object2Sync,
					syncConfig:   syncConfig,
					ctx:          ctx,
				}); err != nil {
				return err
			}

			srcObjKey := buildKey(object2Sync.getGKV(), object2Sync.getSrcObject().GetName(), srcNamespace)
			syncedSrcObjKeys[srcObjKey] = true
		}
	}

	return nil
}

// syncObject syncs object to a user destination namespace.
// Returns error if sync failed in a destination namespace.
func (r *WorkspacesConfigReconciler) syncObject(syncContext *syncContext) error {
	dstObj := syncContext.object2Sync.newDstObject()
	dstObj.SetNamespace(syncContext.dstNamespace)
	// ensure the name is the same as the source object
	dstObj.SetName(syncContext.object2Sync.getSrcObject().GetName())
	// set mandatory labels
	dstObj.SetLabels(utils.MergeMaps(
		[]map[string]string{
			dstObj.GetLabels(),
			{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
			},
		}))

	if err := r.syncObjectIfDiffers(syncContext, dstObj); err != nil {
		logger.Error(err, "Failed to sync object",
			"namespace", syncContext.dstNamespace,
			"kind", gvk2PrintString(syncContext.object2Sync.getGKV()),
			"name", dstObj.GetName())
		return err
	}

	return nil
}

// syncObjectIfDiffers syncs object to a user destination namespace if it differs from the source object.
// Returns error if sync failed in a destination namespace.
func (r *WorkspacesConfigReconciler) syncObjectIfDiffers(
	syncContext *syncContext,
	dstObj client.Object) error {

	existedDstObj, err := r.scheme.New(syncContext.object2Sync.getGKV())
	if err != nil {
		return err
	}
	existedDstObjKey := types.NamespacedName{
		Name:      dstObj.GetName(),
		Namespace: dstObj.GetNamespace(),
	}

	err = r.client.Get(syncContext.ctx, existedDstObjKey, existedDstObj.(client.Object))
	if err == nil {
		srcObj := syncContext.object2Sync.getSrcObject()

		srcObjKey := buildKey(syncContext.object2Sync.getGKV(), srcObj.GetName(), syncContext.srcNamespace)
		dstObjKey := buildKey(syncContext.object2Sync.getGKV(), dstObj.GetName(), syncContext.dstNamespace)

		srcHasBeenChanged := syncContext.syncConfig[srcObjKey] != syncContext.object2Sync.getSrcObjectVersion()
		dstHasBeenChanged := syncContext.syncConfig[dstObjKey] != existedDstObj.(client.Object).GetResourceVersion()

		if srcHasBeenChanged || dstHasBeenChanged {
			// destination object exists, and it differs from the source object,
			// so it will be updated
			if syncContext.object2Sync.hasROSpec() {
				// Skip updating objects with readonly spec.
				// Admin has to re-create them to update just update resource versions
				logger.Info("Object skipped since has readonly spec, re-create it to update",
					"namespace", dstObj.GetNamespace(),
					"kind", gvk2PrintString(syncContext.object2Sync.getGKV()),
					"name", dstObj.GetName())

				r.doUpdateSyncConfig(syncContext, existedDstObj.(client.Object))
				return nil
			} else {
				if isDiff(dstObj, existedDstObj.(client.Object)) {
					if err = r.doUpdateObject(syncContext, dstObj, existedDstObj.(client.Object)); err != nil {
						return err
					}
					r.doUpdateSyncConfig(syncContext, dstObj)
					return nil
				} else {
					// nothing to update objects are equal just update resource versions
					r.doUpdateSyncConfig(syncContext, existedDstObj.(client.Object))
					return nil
				}
			}
		}
	} else if errors.IsNotFound(err) {
		// destination object does not exist, so it will be created
		if err = r.doCreateObject(syncContext, dstObj); err != nil {
			return err
		}
		r.doUpdateSyncConfig(syncContext, dstObj)
		return nil
	} else {
		return err
	}

	return nil
}

// doCreateObject creates object in a user destination namespace.
func (r *WorkspacesConfigReconciler) doCreateObject(
	syncContext *syncContext,
	dstObj client.Object) error {

	if err := r.client.Create(syncContext.ctx, dstObj); err != nil {
		return err
	}

	logger.Info("Object created", "namespace", dstObj.GetNamespace(),
		"kind", gvk2PrintString(syncContext.object2Sync.getGKV()),
		"name", dstObj.GetName())

	return nil
}

// doUpdateObject updates object in a user destination namespace.
func (r *WorkspacesConfigReconciler) doUpdateObject(
	syncContext *syncContext,
	dstObj client.Object,
	existedDstObj client.Object) error {

	// preserve labels and annotations from existed object
	dstObj.SetLabels(utils.MergeMaps(
		[]map[string]string{
			existedDstObj.GetLabels(),
			dstObj.GetLabels(),
		},
	))
	dstObj.SetAnnotations(utils.MergeMaps(
		[]map[string]string{
			existedDstObj.GetAnnotations(),
			dstObj.GetAnnotations(),
		},
	))

	// set the current resource version to update object
	dstObj.SetResourceVersion(existedDstObj.GetResourceVersion())

	if err := r.client.Update(syncContext.ctx, dstObj); err != nil {
		return err
	}

	logger.Info("Object updated", "namespace", dstObj.GetNamespace(),
		"kind", gvk2PrintString(syncContext.object2Sync.getGKV()),
		"name", dstObj.GetName())

	return nil
}

// doUpdateSyncConfig updates sync config with resource versions of synced objects.
func (r *WorkspacesConfigReconciler) doUpdateSyncConfig(syncContext *syncContext, dstObj client.Object) {
	srcObj := syncContext.object2Sync.getSrcObject()

	srcObjKey := buildKey(syncContext.object2Sync.getGKV(), srcObj.GetName(), syncContext.srcNamespace)
	dstObjKey := buildKey(syncContext.object2Sync.getGKV(), dstObj.GetName(), syncContext.dstNamespace)

	syncContext.syncConfig[srcObjKey] = syncContext.object2Sync.getSrcObjectVersion()
	syncContext.syncConfig[dstObjKey] = dstObj.GetResourceVersion()
}

// deleteIfObjectIsObsolete deletes obsolete objects.
// Returns error if delete failed in a destination namespace.
func (r *WorkspacesConfigReconciler) deleteIfObjectIsObsolete(
	objKey string,
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
	syncConfig map[string]string,
	syncedSrcObjKeys map[string]bool) error {

	isSrcObject := getNamespaceItem(objKey) == srcNamespace
	isNotSyncedInDstNamespace := !syncedSrcObjKeys[objKey]

	if isSrcObject && isNotSyncedInDstNamespace {
		objName := getNameItem(objKey)
		gkv := item2gkv(getGkvItem(objKey))

		blueprint, err := r.scheme.New(gkv)
		if err != nil {
			return err
		}

		// delete object from destination namespace
		if err := deploy.DeleteIgnoreIfNotFound(
			ctx,
			r.client,
			types.NamespacedName{
				Name:      objName,
				Namespace: dstNamespace,
			},
			blueprint.(client.Object)); err != nil {
			return err
		}

		dstObjKey := buildKey(gkv, objName, dstNamespace)
		delete(syncConfig, objKey)
		delete(syncConfig, dstObjKey)
	}

	return nil
}

// getSyncConfig returns ConfigMap with synced objects resource versions.
// Returns error if ConfigMap failed to be retrieved.
func (r *WorkspacesConfigReconciler) getSyncConfig(ctx context.Context, namespace string) (*corev1.ConfigMap, error) {
	syncCM := &corev1.ConfigMap{}
	syncCMKey := types.NamespacedName{
		Name:      syncedWorkspacesConfig,
		Namespace: namespace,
	}

	err := r.client.Get(ctx, syncCMKey, syncCM)
	if err != nil {
		if errors.IsNotFound(err) {
			syncCM = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      syncedWorkspacesConfig,
					Namespace: namespace,
					Labels: utils.MergeMaps([]map[string]string{
						wsConfigComponentLabels,
						{constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel()}}),
				},
				Data: map[string]string{},
			}
		} else {
			return nil, err
		}
	} else if syncCM.Data == nil {
		syncCM.Data = map[string]string{}
	}

	return syncCM, nil
}

// buildKey returns a key for ConfigMap.
// The key is built from items of GroupVersionKind, name and namespace.
func buildKey(gvk schema.GroupVersionKind, name string, namespace string) string {
	return fmt.Sprintf("%s.%s.%s", gvk2Item(gvk), name, namespace)
}

func getGkvItem(key string) string {
	splits := strings.Split(key, ".")
	return strings.ReplaceAll(splits[0], "-", ".")
}

func getNameItem(key string) string {
	splits := strings.Split(key, ".")
	return strings.Join(splits[1:len(splits)-1], ".")
}

func getNamespaceItem(key string) string {
	splits := strings.Split(key, ".")
	return splits[len(splits)-1]
}

// gvk2Item returns a key item for GroupVersionKind.
func gvk2Item(gvk schema.GroupVersionKind) string {
	if gvk.Group == "" {
		return fmt.Sprintf("%s_%s", gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("%s_%s_%s", strings.ReplaceAll(gvk.Group, ".", "-"), gvk.Version, gvk.Kind)
}

func item2gkv(item string) schema.GroupVersionKind {
	splits := strings.Split(item, "_")
	if len(splits) == 3 {
		return schema.GroupVersionKind{
			Group:   splits[0],
			Version: splits[1],
			Kind:    splits[2],
		}
	}

	return schema.GroupVersionKind{
		Version: splits[0],
		Kind:    splits[1],
	}
}

// gvk2PrintString returns a string representation of GroupVersionKind.
func gvk2PrintString(gkv schema.GroupVersionKind) string {
	return fmt.Sprintf("%s.%s", gkv.Version, gkv.Kind)
}

func hasWSConfigComponentLabels(obj metav1.Object) bool {
	return obj.GetLabels()[constants.KubernetesComponentLabelKey] == constants.WorkspacesConfig &&
		obj.GetLabels()[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg
}
