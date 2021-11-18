//
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
package deploy

import (
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/util"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ConsoleLinkFinalizerName = "consolelink.finalizers.che.eclipse.org"
	ConsoleLinksResourceName = "consolelinks"
)

func ReconcileConsoleLink(deployContext *DeployContext) (bool, error) {
	if !util.IsOpenShift4 || !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, ConsoleLinksResourceName) {
		// console link is supported only on OpenShift >= 4.2
		logrus.Debug("Console link won't be created. Consolelinks is not supported by OpenShift cluster.")
		return true, nil
	}

	if !deployContext.CheCluster.Spec.Server.TlsSupport {
		// console link is supported only with https
		logrus.Debug("Console link won't be created. HTTP protocol is not supported.")
		return true, nil
	}

	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return createConsoleLink(deployContext)
	}
	return true, nil
}

func ReconcileConsoleLinkFinalizer(deployContext *DeployContext) error {
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return DeleteObjectWithFinalizer(deployContext, client.ObjectKey{Name: DefaultConsoleLinkName()}, &consolev1.ConsoleLink{}, ConsoleLinkFinalizerName)
	}
	return nil
}

func createConsoleLink(deployContext *DeployContext) (bool, error) {
	consoleLinkSpec := getConsoleLinkSpec(deployContext)
	_, err := CreateIfNotExists(deployContext, consoleLinkSpec)
	if err != nil {
		return false, err
	}

	consoleLink := &consolev1.ConsoleLink{}
	exists, err := Get(deployContext, client.ObjectKey{Name: DefaultConsoleLinkName()}, consoleLink)
	if !exists || err != nil {
		return false, err
	}

	// consolelink is for this specific instance of Eclipse Che
	if strings.Index(consoleLink.Spec.Link.Href, deployContext.CheCluster.Spec.Server.CheHost) != -1 {
		err = AppendFinalizer(deployContext, ConsoleLinkFinalizerName)
		return err == nil, err
	}

	return true, nil
}

func getConsoleLinkSpec(deployContext *DeployContext) *consolev1.ConsoleLink {
	cheHost := deployContext.CheCluster.Spec.Server.CheHost
	consoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultConsoleLinkName(),
			Annotations: map[string]string{
				CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: "https://" + cheHost,
				Text: DefaultConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  DefaultConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", cheHost, DefaultConsoleLinkImage()),
			},
		},
	}

	return consoleLink
}
