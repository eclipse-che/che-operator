//
// Copyright (c) 2021 Red Hat, Inc.
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
	"os"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cheFlavor = os.Getenv("CHE_FLAVOR")
)

func OnStartMigration(nonCachingClient client.Client) bool {
	return migrateCheResourcesLabels(nonCachingClient)
}

// migrateCheResourcesLabels searches for objects of different kinds in the cluster,
// that are added by Che admin and used by the Operator in some way.
// When such a resource is found, a new label should be added in order to have the resource cached.
// This should scan all namespaces in order to support all namespaces mode.
// If an error happens, warning is printed and execution continues.
// Returns true if everything is updated without errors, false otherwise.
func migrateCheResourcesLabels(nonCachingClient client.Client) bool {
	noErrors := true

	// Prepare selector
	partOfCheSelectorRequirement, err := labels.NewRequirement(deploy.KubernetesPartOfLabelKey, selection.Equals, []string{deploy.CheEclipseOrg})
	if err != nil {
		logrus.Error(getFailedToCreateSelectorErrorMessage())
		return false
	}
	instanceNotCheFlavorSelectorRequirement, err := labels.NewRequirement(deploy.KubernetesInstanceLabelKey, selection.NotEquals, []string{cheFlavor})
	if err != nil {
		logrus.Error(getFailedToCreateSelectorErrorMessage())
		return false
	}
	objectsToMigrateLabelSelector := labels.NewSelector().Add(*partOfCheSelectorRequirement).Add(*instanceNotCheFlavorSelectorRequirement)
	listOptions := &client.ListOptions{
		LabelSelector: objectsToMigrateLabelSelector,
	}

	// Migrate all config maps
	configMapsList := &corev1.ConfigMapList{}
	err = nonCachingClient.List(context.TODO(), configMapsList, listOptions)
	if err != nil {
		logrus.Warn(getFailedToGetErrorMessageFor("Config Maps"))
		noErrors = false
	}
	if configMapsList.Items != nil {
		for _, cm := range configMapsList.Items {
			cm.ObjectMeta.Labels[deploy.KubernetesInstanceLabelKey] = cheFlavor
			if err := nonCachingClient.Update(context.TODO(), &cm); err != nil {
				logrus.Warn(getFailedToUpdateErrorMessage(cm.GetName(), reflect.TypeOf(cm).Name()))
				noErrors = false
			}
		}
	}

	// Migrate all secrets
	secretsList := &corev1.SecretList{}
	err = nonCachingClient.List(context.TODO(), secretsList, listOptions)
	if err != nil {
		logrus.Warn(getFailedToGetErrorMessageFor("Secrets"))
		noErrors = false
	}
	if secretsList.Items != nil {
		for _, secret := range secretsList.Items {
			secret.ObjectMeta.Labels[deploy.KubernetesInstanceLabelKey] = cheFlavor
			if err := nonCachingClient.Update(context.TODO(), &secret); err != nil {
				logrus.Warn(getFailedToUpdateErrorMessage(secret.GetName(), reflect.TypeOf(secret).Name()))
				noErrors = false
			}
		}
	}

	return noErrors
}

func getFailedToGetErrorMessageFor(item string) string {
	return fmt.Sprintf("Failed to get %s to add %s=%s label. This resources will be ignored by Operator.",
		item, deploy.KubernetesInstanceLabelKey, cheFlavor)
}

func getFailedToUpdateErrorMessage(objectName string, objectKind string) string {
	return fmt.Sprintf("Failed to update %s '%s' with label %s=%s. This resource will be ignored by Operator.",
		objectKind, objectName, deploy.KubernetesInstanceLabelKey, cheFlavor)
}

func getFailedToCreateSelectorErrorMessage() string {
	return "Failed to create selector for resources migration. Unable to perform resources migration."
}
