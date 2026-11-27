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

package editorsdefinitions

import (
	"os"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

var _ = Describe("EditorsDefinitionsReconciler", func() {
	var cheCtx *chetypes.CheContext

	Context("readEditorDefinitions", func() {
		It("should read all devfiles from the editors definitions directory", func() {
			editorDefinitions, err := readEditorDefinitions()
			Expect(err).To(Succeed())
			Expect(editorDefinitions).To(HaveLen(2))
			Expect(editorDefinitions).To(HaveKey("devfile-1.yaml"))
			Expect(editorDefinitions).To(HaveKey("devfile-2.yaml"))
		})

		It("should skip broken and empty devfiles", func() {
			editorDefinitions, err := readEditorDefinitions()
			Expect(err).To(Succeed())
			Expect(editorDefinitions).NotTo(HaveKey("devfile-broken.yaml"))
			Expect(editorDefinitions).NotTo(HaveKey("devfile-empy.yaml"))
		})

		Context("when related image env variables are set", func() {
			BeforeEach(func() {
				Expect(os.Setenv("RELATED_IMAGE_editor_definition_che_code_1_2_3_component_a", "image-new-1_2_3-a")).To(Succeed())
				Expect(os.Setenv("RELATED_IMAGE_editor_definition_che_code_2022_1_component_a", "image-new-2022_1-a")).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.Unsetenv("RELATED_IMAGE_editor_definition_che_code_1_2_3_component_a")).To(Succeed())
				Expect(os.Unsetenv("RELATED_IMAGE_editor_definition_che_code_2022_1_component_a")).To(Succeed())
			})

			It("should substitute container images in editor definitions", func() {
				editorDefinitions, err := readEditorDefinitions()
				Expect(err).To(Succeed())
				Expect(editorDefinitions).To(HaveLen(2))

				var devfile map[string]interface{}
				Expect(yaml.Unmarshal(editorDefinitions["devfile-1.yaml"], &devfile)).To(Succeed())

				components := devfile["components"].([]interface{})
				Expect(components).To(HaveLen(3))

				container := components[0].(map[string]interface{})["container"].(map[string]interface{})
				Expect(container["image"]).To(Equal("image-new-1_2_3-a"))

				container = components[1].(map[string]interface{})["container"].(map[string]interface{})
				Expect(container["image"]).To(Equal("image-b"))

				_, ok := components[2].(map[string]interface{})["container"].(map[string]interface{})
				Expect(ok).To(BeFalse())

				Expect(yaml.Unmarshal(editorDefinitions["devfile-2.yaml"], &devfile)).To(Succeed())
				components = devfile["components"].([]interface{})
				container = components[0].(map[string]interface{})["container"].(map[string]interface{})
				Expect(container["image"]).To(Equal("image-new-2022_1-a"))
			})
		})
	})

	Context("Reconcile", func() {
		It("should create the editors definitions ConfigMap", func() {
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx = test.NewCheContextBuilder().Build()

			reconciler := NewReconciler()
			Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

			configMap := &corev1.ConfigMap{}
			key := types.NamespacedName{Name: configMapName, Namespace: cheCtx.CheCluster.Namespace}
			Expect(test.ExpectFound(key, configMap)).To(Succeed())
			Expect(configMap.Labels).To(Equal(chelabels.GetStandardLabels(componentName)))
			Expect(configMap.Data).To(HaveKey("devfile-1.yaml"))
			Expect(configMap.Data).To(HaveKey("devfile-2.yaml"))
		})
	})
})
