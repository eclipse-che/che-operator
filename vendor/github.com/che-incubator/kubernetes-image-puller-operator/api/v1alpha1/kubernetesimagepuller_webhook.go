//
// Copyright (c) 2012-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package v1alpha1

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *KubernetesImagePuller) SetupWebhookWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().Register("/validate-che-eclipse-org-v1alpha1-kubernetesimagepuller", &webhook.Admission{Handler: &validationHandler{k8sClient: mgr.GetClient()}})
	return nil
}

// +kubebuilder:webhook:path=/validate-che-eclipse-org-v1alpha1-kubernetesimagepuller,mutating=false,failurePolicy=fail,sideEffects=None,groups=che.eclipse.org,resources=kubernetesimagepullers,verbs=create,versions=v1alpha1,name=vkubernetesimagepuller.kb.io,admissionReviewVersions={v1,v1beta1}

type validationHandler struct {
	k8sClient client.Client
}

func (v *validationHandler) Handle(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
	imagePullers := &KubernetesImagePullerList{}
	err := v.k8sClient.List(ctx, imagePullers, &client.ListOptions{Namespace: req.Namespace})
	if err != nil {
		return webhook.Denied(err.Error())
	}

	if len(imagePullers.Items) > 0 {
		return webhook.Denied("only one KubernetesImagePuller is allowed per namespace")
	}
	return webhook.Allowed("there are no KubernetesImagePuller resources in this namespace")
}
