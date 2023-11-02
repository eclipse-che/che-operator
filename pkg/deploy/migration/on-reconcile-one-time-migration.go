// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	oauthv1 "github.com/openshift/api/oauth/v1"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Migrator struct {
	deploy.Reconcilable

	migrationDone bool
}

func NewMigrator() *Migrator {
	return &Migrator{
		migrationDone: false,
	}
}

func (m *Migrator) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if m.migrationDone {
		return reconcile.Result{}, true, nil
	}

	done, err := m.migrate(ctx)
	if done && err == nil {
		m.migrationDone = true
		// Give some time for the migration resources to be flushed and rerun reconcile
		return reconcile.Result{RequeueAfter: 5 * time.Second}, false, err
	}
	return reconcile.Result{}, done, err
}

func (m *Migrator) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (m *Migrator) migrate(ctx *chetypes.DeployContext) (bool, error) {
	if err := addPartOfCheLabeltoUserDefinedObjects(ctx); err != nil {
		return false, err
	}

	cheFlavor := defaults.GetCheFlavor()
	if err := addPartOfCheLabelForObjectsWithLabel(ctx, constants.KubernetesInstanceLabelKey, cheFlavor); err != nil {
		return false, err
	}

	if err := addPartOfCheLabelForObjectsWithLabel(ctx, "app", cheFlavor); err != nil {
		return false, err
	}

	return true, nil
}

// addPartOfCheLabeltoUserDefinedObjects processes the following objects to add 'app.kubernetes.io/part-of=che.eclipse.org' label:
// - spec.server.cheClusterRoles
// - spec.server.cheWorkspaceClusterRole
// - spec.server.serverTrustStoreConfigMapName
// - spec.server.proxy.credentialssecretname
// - spec.networking.tlsSecretName
// Note, most of the objects above are autogenerated and do not require any migration,
// but to handle the case when some were created manually or operator updated, the check is done here.
func addPartOfCheLabeltoUserDefinedObjects(ctx *chetypes.DeployContext) error {
	if !infrastructure.IsOpenShift() {
		// Kubernetes only
		tlsSecretName := utils.GetValue(ctx.CheCluster.Spec.Networking.TlsSecretName, constants.DefaultCheTLSSecretName)
		if err := addPartOfCheLabelToSecret(ctx, tlsSecretName); err != nil {
			return err
		}
	}

	// TLS
	if err := addPartOfCheLabelToSecret(ctx, constants.DefaultSelfSignedCertificateSecretName); err != nil {
		return err
	}

	// Proxy credentials
	if ctx.CheCluster.Spec.Components.CheServer.Proxy != nil {
		proxyCredentialsSecret := utils.GetValue(ctx.CheCluster.Spec.Components.CheServer.Proxy.CredentialsSecretName, constants.DefaultProxyCredentialsSecret)
		if err := addPartOfCheLabelToSecret(ctx, proxyCredentialsSecret); err != nil {
			return err
		}
	}

	// Legacy config map with additional CA certificates
	if err := addPartOfCheLabelToConfigMap(ctx, constants.DefaultServerTrustStoreConfigMapName); err != nil {
		return err
	}

	// Config map with CA certificates for git
	if ctx.CheCluster.Spec.DevEnvironments.TrustedCerts != nil {
		gitTrustedCertsConfigMapName := utils.GetValue(ctx.CheCluster.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName, constants.DefaultGitSelfSignedCertsConfigMapName)
		if err := addPartOfCheLabelToConfigMap(ctx, gitTrustedCertsConfigMapName); err != nil {
			return err
		}
	}

	return nil
}

func addPartOfCheLabelToSecret(ctx *chetypes.DeployContext, secretName string) error {
	return addPartOfCheLabelToObject(ctx, secretName, &corev1.Secret{})
}

func addPartOfCheLabelToConfigMap(ctx *chetypes.DeployContext, configMapName string) error {
	return addPartOfCheLabelToObject(ctx, configMapName, &corev1.ConfigMap{})
}

// addPartOfCheLabelToObject adds 'app.kubernetes.io/part-of=che.eclipse.org' label to the object with given name to be cached by operator's k8s client.
// As the function doesn't know the kind of the object with given name an empty object should be passed,
// for example: addPartOfCheLabelToObject(ctx, "my-secret", &corev1.Secret{})
func addPartOfCheLabelToObject(ctx *chetypes.DeployContext, objectName string, obj client.Object) error {
	// Check if the object is already migrated
	if exists, _ := deploy.GetNamespacedObject(ctx, objectName, obj); exists {
		// Default client sees the object in cache, no need in adding anything
		return nil
	}

	err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: objectName}, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			// The object doesn't exist in cluster, nothing to do
			return nil
		}
		return err
	}

	if err := ctx.ClusterAPI.NonCachingClient.Update(context.TODO(), setPartOfLabel(obj)); err != nil {
		return err
	}
	logrus.Info(getObjectMigratedMessage(obj))

	return nil
}

func setPartOfLabel(obj client.Object) client.Object {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[constants.KubernetesPartOfLabelKey] = constants.CheEclipseOrg
	obj.SetLabels(labels)
	return obj
}

// addPartOfCheLabelForObjectsWithLabel searches for objects in Che installation namespace,
// that have given label and adds 'app.kubernetes.io/part-of=che.eclipse.org'
func addPartOfCheLabelForObjectsWithLabel(ctx *chetypes.DeployContext, labelKey string, labelValue string) error {
	// Prepare selector for all instance=che objects in the installation namespace
	instanceCheSelectorRequirement, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelValue})
	if err != nil {
		logrus.Error(getFailedToCreateSelectorErrorMessage())
		return err
	}
	// Do not migrate already migrated objects
	notPartOfCheSelectorRequirement, err := labels.NewRequirement(constants.KubernetesPartOfLabelKey, selection.NotEquals, []string{constants.CheEclipseOrg})
	if err != nil {
		logrus.Error(getFailedToCreateSelectorErrorMessage())
		return err
	}
	objectsToMigrateLabelSelector := labels.NewSelector().
		Add(*instanceCheSelectorRequirement).
		Add(*notPartOfCheSelectorRequirement)
	listOptions := &client.ListOptions{
		LabelSelector: objectsToMigrateLabelSelector,
		Namespace:     ctx.CheCluster.GetNamespace(),
	}

	// This list should be based on the list from the cache function (see NewCache filed of the managar in main.go)
	kindsToMigrate := []client.ObjectList{
		&appsv1.DeploymentList{},
		&corev1.PodList{},
		&batchv1.JobList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.ServiceAccountList{},
		&rbacv1.RoleList{},
		&rbacv1.RoleBindingList{},
		&rbacv1.ClusterRoleList{},
		&rbacv1.ClusterRoleBindingList{},
		&corev1.PersistentVolumeClaimList{},
	}
	if infrastructure.IsOpenShift() {
		kindsToMigrate = append(kindsToMigrate, &routev1.RouteList{})
		kindsToMigrate = append(kindsToMigrate, &oauthv1.OAuthClientList{})
	} else {
		kindsToMigrate = append(kindsToMigrate, &networkingv1.IngressList{})
	}

	for _, listToGet := range kindsToMigrate {
		if err := addPartOfCheLabelToObjectsBySelector(ctx, listOptions, listToGet); err != nil {
			return err
		}
	}

	return nil
}

// addPartOfCheLabelToObjectsBySelector adds 'app.kubernetes.io/part-of=che.eclipse.org' label to all objects
// of given objectsList kind that match the provided in listOptions selector and namespace.
func addPartOfCheLabelToObjectsBySelector(ctx *chetypes.DeployContext, listOptions *client.ListOptions, objectsList client.ObjectList) error {
	if err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), objectsList, listOptions); err != nil {
		logrus.Warnf("Failed to get %s to add %s label", objectsList.GetObjectKind().GroupVersionKind().Kind, constants.KubernetesPartOfLabelKey)
		return err
	}

	objects, err := meta.ExtractList(objectsList)
	if err != nil {
		return err
	}

	for _, runtimeObj := range objects {
		obj := setPartOfLabel(runtimeObj.(client.Object))
		if err := ctx.ClusterAPI.NonCachingClient.Update(context.TODO(), obj); err != nil {
			return err
		}
		logrus.Info(getObjectMigratedMessage(obj))
	}

	return nil
}

func getFailedToCreateSelectorErrorMessage() string {
	return "Failed to create selector for resources migration. Unable to perform resources migration."
}

func getObjectMigratedMessage(obj client.Object) string {
	return fmt.Sprintf("Added '%s=%s' label to %s object of %s kind",
		constants.KubernetesPartOfLabelKey, constants.CheEclipseOrg, obj.GetName(), deploy.GetObjectType(obj))
}
