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

package defaultsupdater

import (
	"encoding/json"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("DefaultsUpdaterReconciler", func() {
	var cheCtx *chetypes.CheContext

	BeforeEach(func() {
		test.Initialize(infrastructure.Kubernetes)
	})

	It("should append CHOWN to a user-configured capabilities list and mark the task done", func() {
		// CheURL and CheVersion are set, so Che is being upgraded, not installed
		cheCluster := &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Status: chev2.CheClusterStatus{
				CheURL:     "https://che-host",
				CheVersion: "7.50.0",
			},
			Spec: chev2.CheClusterSpec{
				DevEnvironments: chev2.CheClusterDevEnvironments{
					ContainerRunConfiguration: &chev2.ContainerRunConfiguration{
						ContainerSecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{"NET_BIND_SERVICE"},
							},
						},
					},
				},
			},
		}
		cheCtx = test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

		reconciler := NewReconciler()
		_, done, err := reconciler.Reconcile(cheCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(done).To(BeTrue())

		capabilities := cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration.ContainerSecurityContext.Capabilities.Add
		Expect(capabilities).To(Equal([]corev1.Capability{"NET_BIND_SERVICE", "CHOWN"}))

		// the task is recorded as done in the annotation
		fields := map[string]string{}
		Expect(json.Unmarshal([]byte(cheCtx.CheCluster.GetAnnotations()[cheClusterDefaultsUpdateAnnotation]), &fields)).To(Succeed())
		Expect(fields["spec.devEnvironments.containerRunConfiguration.containerSecurityContext.capabilities.add"]).To(Equal("true"))

		// the second reconcile does not append CHOWN again
		_, done, err = reconciler.Reconcile(cheCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(done).To(BeTrue())

		capabilities = cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration.ContainerSecurityContext.Capabilities.Add
		Expect(capabilities).To(Equal([]corev1.Capability{"NET_BIND_SERVICE", "CHOWN"}))
	})
})
