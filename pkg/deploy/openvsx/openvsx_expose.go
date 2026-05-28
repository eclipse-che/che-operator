//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package openvsx

import (
	"sort"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	OpenVSXIngressName = "openvsx"
)

var serverPaths = []string{
	"/api",
	"/user",
	"/login",
	"/logout",
	"/oauth2",
	"/login-providers",
	"/admin",
	"/actuator",
	"/documents",
	"/swagger-ui",
	"/v3",
}

type OpenVSXExposeReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXExposeReconciler() *OpenVSXExposeReconciler {
	return &OpenVSXExposeReconciler{}
}

func (r *OpenVSXExposeReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsOpenVSXOperandEnabled() {
		_, _ = deploy.DeleteNamespacedObject(ctx, OpenVSXIngressName, &networking.Ingress{})

		if ctx.CheCluster.Status.OpenVSXURL != "" {
			ctx.CheCluster.Status.OpenVSXURL = ""
			err := deploy.UpdateCheCRStatus(ctx, "OpenVSXURL", "")
			return reconcile.Result{}, err == nil, err
		}

		return reconcile.Result{}, true, nil
	}

	hostname := r.getHostname(ctx)
	if hostname == "" {
		return reconcile.Result{Requeue: true}, false, nil
	}

	done, err := r.syncIngress(ctx, hostname)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.updateStatus(ctx, hostname)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXExposeReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (r *OpenVSXExposeReconciler) getHostname(ctx *chetypes.DeployContext) string {
	baseDomain := ctx.CheCluster.Spec.Networking.Domain
	if baseDomain == "" && ctx.CheHost != "" {
		parts := strings.SplitN(ctx.CheHost, ".", 2)
		if len(parts) == 2 {
			baseDomain = parts[1]
		}
	}

	if baseDomain == "" {
		return ""
	}

	return "openvsx-" + ctx.CheCluster.Namespace + "." + baseDomain
}

func (r *OpenVSXExposeReconciler) syncIngress(ctx *chetypes.DeployContext, hostname string) (bool, error) {
	ingress := r.getIngressSpec(ctx, hostname)
	return deploy.Sync(ctx, ingress, deploy.IngressDiffOpts)
}

func (r *OpenVSXExposeReconciler) getIngressSpec(ctx *chetypes.DeployContext, hostname string) *networking.Ingress {
	labels := deploy.GetLabels(OpenVSXIngressName)
	for k, v := range ctx.CheCluster.Spec.Networking.Labels {
		labels[k] = v
	}

	annotations := r.getAnnotations(ctx)
	pathType := networking.PathTypePrefix

	serverBackend := networking.IngressBackend{
		Service: &networking.IngressServiceBackend{
			Name: constants.OpenVSXServerName,
			Port: networking.ServiceBackendPort{
				Number: 8080,
			},
		},
	}

	webuiBackend := networking.IngressBackend{
		Service: &networking.IngressServiceBackend{
			Name: constants.OpenVSXWebUIName,
			Port: networking.ServiceBackendPort{
				Number: 3000,
			},
		},
	}

	var paths []networking.HTTPIngressPath
	for _, p := range serverPaths {
		paths = append(paths, networking.HTTPIngressPath{
			Path:     p,
			PathType: &pathType,
			Backend:  serverBackend,
		})
	}
	paths = append(paths, networking.HTTPIngressPath{
		Path:     "/",
		PathType: &pathType,
		Backend:  webuiBackend,
	})

	ingressClassName := ctx.CheCluster.Spec.Networking.IngressClassName
	if ingressClassName == "" {
		ingressClassName = annotations["kubernetes.io/ingress.class"]
	}
	delete(annotations, "kubernetes.io/ingress.class")

	ingress := &networking.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: networking.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        OpenVSXIngressName,
			Namespace:   ctx.CheCluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networking.IngressSpec{
			IngressClassName: pointer.String(ingressClassName),
			TLS: []networking.IngressTLS{
				{
					Hosts:      []string{hostname},
					SecretName: ctx.CheCluster.Spec.Networking.TlsSecretName,
				},
			},
			Rules: []networking.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: paths,
						},
					},
				},
			},
		},
	}

	return ingress
}

func (r *OpenVSXExposeReconciler) getAnnotations(ctx *chetypes.DeployContext) map[string]string {
	annotations := map[string]string{}
	if len(ctx.CheCluster.Spec.Networking.Annotations) > 0 {
		for k, v := range ctx.CheCluster.Spec.Networking.Annotations {
			annotations[k] = v
		}
	} else {
		for k, v := range deploy.DefaultIngressAnnotations {
			annotations[k] = v
		}
		annotations["nginx.ingress.kubernetes.io/proxy-buffer-size"] = "16k"
	}

	annotationsKeys := make([]string, 0, len(annotations))
	for k := range annotations {
		annotationsKeys = append(annotationsKeys, k)
	}
	if len(annotationsKeys) > 0 {
		sort.Strings(annotationsKeys)
		data := ""
		for _, k := range annotationsKeys {
			data += k + ":" + annotations[k] + ","
		}
		if test.IsTestMode() {
			annotations[constants.CheEclipseOrgManagedAnnotationsDigest] = "0000"
		} else {
			annotations[constants.CheEclipseOrgManagedAnnotationsDigest] = utils.ComputeHash256([]byte(data))
		}
	}

	return annotations
}

func (r *OpenVSXExposeReconciler) updateStatus(ctx *chetypes.DeployContext, hostname string) (bool, error) {
	openVSXURL := "https://" + hostname

	if openVSXURL != ctx.CheCluster.Status.OpenVSXURL {
		ctx.CheCluster.Status.OpenVSXURL = openVSXURL
		if err := deploy.UpdateCheCRStatus(ctx, "status: OpenVSX URL", openVSXURL); err != nil {
			return false, err
		}
	}

	return true, nil
}
