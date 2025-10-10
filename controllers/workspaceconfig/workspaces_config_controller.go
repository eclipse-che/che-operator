//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package workspace_config

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	networkingv1 "k8s.io/api/networking/v1"

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
)

const (
	syncedWorkspacesConfig = "sync-workspaces-config"
	syncRetainAnnotation   = "che.eclipse.org/sync-retain"
)

type WorkspacesConfigReconciler struct {
	scheme                        *runtime.Scheme
	client                        client.Client
	clientWrapper                 *k8sclient.K8sClientWrapper
	nonCachedClientWrapper        *k8sclient.K8sClientWrapper
	namespaceCache                *namespacecache.NamespaceCache
	labelsToRemoveBeforeSync      []*regexp.Regexp
	annotationsToRemoveBeforeSync []*regexp.Regexp
}

type Object2Sync interface {
	getGKV() schema.GroupVersionKind
	hasROSpec() bool
	getSrcObject() client.Object
	getSrcObjectVersion() string
	newDstObject() client.Object
	defaultRetention() bool
}

type syncContext struct {
	dstNamespace string
	srcNamespace string
	ctx          context.Context
	object2Sync  Object2Sync
	syncConfig   map[string]string
}

const (
	envLabelsToRemoveBeforeSync      = "CHE_OPERATOR_WORKSPACES_CONFIG_CONTROLLER_LABELS_TO_REMOVE_BEFORE_SYNC_REGEXP"
	envAnnotationsToRemoveBeforeSync = "CHE_OPERATOR_WORKSPACES_CONFIG_CONTROLLER_ANNOTATIONS_TO_REMOVE_BEFORE_SYNC_REGEXP"
)

var (
	logger                  = ctrl.Log.WithName("workspaces-config")
	wsConfigComponentLabels = map[string]string{
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
	}
	wsConfigComponentSelector = labels.SelectorFromSet(wsConfigComponentLabels)
)

func NewWorkspacesConfigReconciler(
	cli client.Client,
	nonCachedCli client.Client,
	scheme *runtime.Scheme,
	namespaceCache *namespacecache.NamespaceCache) *WorkspacesConfigReconciler {

	labelsToRemoveBeforeSyncAsString := os.Getenv(envLabelsToRemoveBeforeSync)
	annotationsToRemoveBeforeSyncAsString := os.Getenv(envAnnotationsToRemoveBeforeSync)

	var labelsToRemoveBeforeSync []*regexp.Regexp
	for _, label := range strings.Split(labelsToRemoveBeforeSyncAsString, ",") {
		label = strings.TrimSpace(label)
		if label != "" {
			labelsToRemoveBeforeSync = append(labelsToRemoveBeforeSync, regexp.MustCompile(label))
		}
	}

	var annotationsToRemoveBeforeSync []*regexp.Regexp
	for _, annotation := range strings.Split(annotationsToRemoveBeforeSyncAsString, ",") {
		annotation = strings.TrimSpace(annotation)
		if annotation != "" {
			annotationsToRemoveBeforeSync = append(annotationsToRemoveBeforeSync, regexp.MustCompile(annotation))
		}
	}

	return &WorkspacesConfigReconciler{
		scheme:                        scheme,
		client:                        cli,
		clientWrapper:                 k8sclient.NewK8sClient(cli, scheme),
		nonCachedClientWrapper:        k8sclient.NewK8sClient(nonCachedCli, scheme),
		namespaceCache:                namespaceCache,
		labelsToRemoveBeforeSync:      labelsToRemoveBeforeSync,
		annotationsToRemoveBeforeSync: annotationsToRemoveBeforeSync,
	}
}

func (r *WorkspacesConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&corev1.PersistentVolumeClaim{}, r.watchRules(ctx, true, true)).
		Watches(&corev1.Secret{}, r.watchRules(ctx, true, true)).
		Watches(&corev1.ConfigMap{}, r.watchRules(ctx, true, true)).
		Watches(&corev1.ResourceQuota{}, r.watchRules(ctx, false, true)).
		Watches(&corev1.LimitRange{}, r.watchRules(ctx, false, true)).
		Watches(&corev1.ServiceAccount{}, r.watchRules(ctx, false, true)).
		Watches(&rbacv1.Role{}, r.watchRules(ctx, false, true)).
		Watches(&rbacv1.RoleBinding{}, r.watchRules(ctx, false, true)).
		Watches(&networkingv1.NetworkPolicy{}, r.watchRules(ctx, false, true))

	if infrastructure.IsOpenShift() {
		bld.Watches(&templatev1.Template{}, r.watchRules(ctx, true, false))
	}

	// Use controller.TypedOptions to allow to configure 2 controllers for same object being reconciled
	return bld.WithOptions(
		controller.TypedOptions[reconcile.Request]{
			SkipNameValidation: pointer.Bool(true),
		}).Complete(r)
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

	if err = r.syncNamespace(ctx, checluster.Namespace, req.Name); err != nil {
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
		func(context context.Context, obj client.Object) []reconcile.Request {
			var eventRules []namespacecache.EventRule

			if cheNamespaceRule {
				eventRules = append(eventRules,
					namespacecache.EventRule{
						// reconcile rule when workspace config is modified in a che namespace
						// to update the config in all users` namespaces
						Check: func(o metav1.Object) bool {
							cheCluster, _ := deploy.FindCheClusterCRInNamespace(r.client, o.GetNamespace())
							return hasWSConfigComponentLabels(o) && cheCluster != nil
						},
						Namespaces: func(o metav1.Object) []string { return r.namespaceCache.GetAllKnownNamespaces() },
					},
				)
			}

			if userNamespaceRule {
				eventRules = append(eventRules,
					namespacecache.EventRule{
						// reconcile rule when workspace config is modified in a user namespace
						// to revert the config
						Check: func(o metav1.Object) bool {
							workspaceInfo, _ := r.namespaceCache.GetNamespaceInfo(ctx, o.GetNamespace())
							return hasWSConfigComponentLabels(o) &&
								o.GetName() != syncedWorkspacesConfig &&
								workspaceInfo != nil &&
								workspaceInfo.IsWorkspaceNamespace
						},
						Namespaces: func(o metav1.Object) []string { return []string{o.GetNamespace()} },
					},
				)
			}

			return namespacecache.AsReconcileRequestsForNamespaces(obj, eventRules)
		})
}

// syncNamespace sync user namespace.
// Iterates over all objects in the source namespace labeled as `app.kubernetes.io/component=workspaces-config`
// and syncs them to the target user namespace.
func (r *WorkspacesConfigReconciler) syncNamespace(
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
				if err = r.clientWrapper.Create(
					ctx,
					syncConfig,
					nil,
				); err != nil {
					logger.Error(err, "Failed to workspace create sync config", "namespace", dstNamespace)
				}
			} else {
				if err = r.clientWrapper.Sync(
					ctx,
					syncConfig,
					nil,
				); err != nil {
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

	objsList := []client.ObjectList{
		&corev1.ConfigMapList{},
		&corev1.SecretList{},
		&corev1.PersistentVolumeClaimList{},
	}
	for _, objList := range objsList {
		if err = r.syncObjectsList(
			ctx,
			srcNamespace,
			dstNamespace,
			syncConfig.Data,
			syncedSrcObjKeys,
			objList,
		); err != nil {
			return err
		}
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

// syncObjectsList syncs objects labeled as `app.kubernetes.io/component=workspaces-config`
// from source namespace to a target user namespace.
func (r *WorkspacesConfigReconciler) syncObjectsList(
	ctx context.Context,
	srcNamespace string,
	dstNamespace string,
	syncConfig map[string]string,
	syncedSrcObjKeys map[string]bool,
	srcObjList client.ObjectList) error {

	opts := &client.ListOptions{
		Namespace:     srcNamespace,
		LabelSelector: wsConfigComponentSelector,
	}

	srcObjs, err := r.clientWrapper.List(ctx, srcObjList, opts)
	if err != nil {
		return err
	}

	for _, srcObj := range srcObjs {
		obj2Sync := createObject2SyncFromObject(srcObj.(client.Object))
		if obj2Sync == nil {
			logger.Info("Object skipped since has unsupported kind",
				"kind", gvk2PrintString(srcObj.GetObjectKind().GroupVersionKind()))
			break
		}

		if err = r.syncObject(
			&syncContext{
				dstNamespace: dstNamespace,
				srcNamespace: srcNamespace,
				object2Sync:  obj2Sync,
				syncConfig:   syncConfig,
				ctx:          ctx,
			}); err != nil {
			return err
		}

		srcObjKey := buildKey(obj2Sync.getGKV(), obj2Sync.getSrcObject().GetName(), srcNamespace)
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

	templateList := &templatev1.TemplateList{}
	opts := &client.ListOptions{
		Namespace:     srcNamespace,
		LabelSelector: wsConfigComponentSelector,
	}

	templates, err := r.clientWrapper.List(ctx, templateList, opts)
	if err != nil {
		return nil
	}

	nsInfo, err := r.namespaceCache.GetNamespaceInfo(ctx, dstNamespace)
	if err != nil {
		return nil
	}

	for _, template := range templates {
		for _, object := range template.(*templatev1.Template).Objects {
			object2Sync, err := createObject2SyncFromRawData(object.Raw, nsInfo.Username, dstNamespace)
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

	// Removes labels that are not needed in the destination namespace
	for _, l := range r.labelsToRemoveBeforeSync {
		for label := range dstObj.GetLabels() {
			if l.MatchString(label) {
				delete(dstObj.GetLabels(), label)
			}
		}
	}

	// Removes annotations that are not needed in the destination namespace
	for _, a := range r.annotationsToRemoveBeforeSync {
		for annotation := range dstObj.GetAnnotations() {
			if a.MatchString(annotation) {
				delete(dstObj.GetAnnotations(), annotation)
			}
		}
	}

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

	exists, err := r.clientWrapper.GetIgnoreNotFound(syncContext.ctx, existedDstObjKey, existedDstObj.(client.Object))
	if exists {
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
					if err = r.doUpdateObject(syncContext, dstObj); err != nil {
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
	} else {
		if err == nil {
			// destination object does not exist, so it will be created
			if err = r.doCreateObject(syncContext, dstObj); err != nil {
				return err
			}
			r.doUpdateSyncConfig(syncContext, dstObj)
			return nil
		} else {
			return err
		}
	}

	return nil
}

// doCreateObject creates object in a user destination namespace.
func (r *WorkspacesConfigReconciler) doCreateObject(
	syncContext *syncContext,
	dstObj client.Object) error {

	err := r.clientWrapper.Create(syncContext.ctx, dstObj, nil)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}

		// AlreadyExists Error might happen if object already exists and doesn't contain
		// `app.kubernetes.io/part-of=che.eclipse.org` label (is not cached)
		// 1. Delete the object from a destination namespace using non-cached client
		// 2. Create the object again using cached client

		namespacedName := types.NamespacedName{
			Name:      dstObj.GetName(),
			Namespace: dstObj.GetNamespace(),
		}

		if retain, err := r.shouldRetain(
			syncContext.ctx,
			namespacedName,
			syncContext.object2Sync.getGKV(),
			r.nonCachedClientWrapper,
		); err != nil {
			return err
		} else if retain {
			// We have to delete and create the object again
			// From the other hand it must be retained
			return fmt.Errorf(
				"cannot sync object %s/%s: it must be deleted and recreated, yet retention is required",
				dstObj.GetNamespace(),
				dstObj.GetName(),
			)
		}

		if err = r.nonCachedClientWrapper.DeleteByKeyIgnoreNotFound(
			syncContext.ctx,
			namespacedName,
			dstObj,
		); err != nil {
			return err
		}

		if err = r.clientWrapper.Create(syncContext.ctx, dstObj, nil); err != nil {
			return err
		}
	}

	return nil
}

// doUpdateObject updates object in a user destination namespace.
func (r *WorkspacesConfigReconciler) doUpdateObject(
	syncContext *syncContext,
	dstObj client.Object) error {

	if err := r.clientWrapper.Sync(
		syncContext.ctx,
		dstObj,
		nil,
		&k8sclient.SyncOptions{MergeAnnotations: true, MergeLabels: true, SuppressDiff: true},
	); err != nil {
		return err
	}

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

	isSrcObjectKey := getNamespaceItem(objKey) == srcNamespace
	isNotSyncedInDstNamespace := !syncedSrcObjKeys[objKey]

	// This can happen when the source object is deleted, but
	// record still present in sync config
	if isSrcObjectKey && isNotSyncedInDstNamespace {
		objName := getNameItem(objKey)
		gkv := item2gkv(getGkvItem(objKey))

		namespacedName := types.NamespacedName{
			Name:      objName,
			Namespace: dstNamespace,
		}

		retain, err := r.shouldRetain(
			ctx,
			namespacedName,
			gkv,
			r.clientWrapper,
		)
		if err != nil {
			return err
		}

		if !retain {
			blueprint, err := r.scheme.New(gkv)
			if err != nil {
				return err
			}

			// delete object from destination namespace
			if err = r.clientWrapper.DeleteByKeyIgnoreNotFound(
				ctx,
				namespacedName,
				blueprint.(client.Object)); err != nil {
				return err
			}
		} else {
			logger.Info(
				"Object retained in destination namespace; deletion skipped",
				"name", objName,
				"namespace", dstNamespace,
			)
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

	exists, err := r.clientWrapper.GetIgnoreNotFound(ctx, syncCMKey, syncCM)
	if exists {
		if syncCM.Data == nil {
			syncCM.Data = map[string]string{}
		}
	} else {
		if err == nil {
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
	}

	return syncCM, nil
}

// shouldRetain returns true if object in the destination namespace
// should be retained if source one is deleted.
func (r *WorkspacesConfigReconciler) shouldRetain(
	ctx context.Context,
	key client.ObjectKey,
	gkv schema.GroupVersionKind,
	clientWrapper *k8sclient.K8sClientWrapper,
) (bool, error) {
	blueprint, err := r.scheme.New(gkv)
	if err != nil {
		return false, err
	}

	exists, err := clientWrapper.GetIgnoreNotFound(ctx, key, blueprint.(client.Object))
	if !exists {
		return false, err
	}

	retainAnnotation := blueprint.(metav1.Object).GetAnnotations()[syncRetainAnnotation]
	if retainAnnotation != "" {
		return strconv.ParseBool(retainAnnotation)
	}

	obj2Sync := createObject2SyncFromObject(blueprint.(client.Object))
	return obj2Sync.defaultRetention(), nil
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
