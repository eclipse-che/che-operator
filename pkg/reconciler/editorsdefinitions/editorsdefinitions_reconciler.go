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
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"

	"github.com/eclipse-che/che-operator/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	componentName = "editor-definition"
	configMapName = "editors-definitions"

	editorDefinitionEnvNamePrefix = "RELATED_IMAGE_editor_definition"
)

var (
	logger = ctrl.Log.WithName("editorsdefinitions")

	editorsDefinitionsDir = "/tmp/editors-definitions"
)

type EditorsDefinitionsReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *EditorsDefinitionsReconciler {
	return &EditorsDefinitionsReconciler{}
}

func (p *EditorsDefinitionsReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := p.syncAll(cheCtx); err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to reconcile Editor definitions: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (p *EditorsDefinitionsReconciler) Finalize(_ *chetypes.CheContext) error {
	return nil
}

func (p *EditorsDefinitionsReconciler) syncAll(cheCtx *chetypes.CheContext) error {
	editorDefinitions, err := readEditorDefinitions()
	if err != nil {
		return fmt.Errorf("failed to read editors definitions: %w", err)
	}

	return syncEditorDefinitions(cheCtx, editorDefinitions)
}

func readEditorDefinitions() (map[string][]byte, error) {
	editorDefinitions := make(map[string][]byte)

	files, err := os.ReadDir(editorsDefinitionsDir)
	if err != nil {
		return editorDefinitions, err
	}

	for _, file := range files {
		if !file.IsDir() {
			fileName := file.Name()

			editorContent, err := os.ReadFile(filepath.Join(editorsDefinitionsDir, fileName))
			if err != nil {
				return editorDefinitions, err
			}

			var devfile map[string]interface{}
			err = yaml.Unmarshal(editorContent, &devfile)
			if err != nil {
				logger.Error(err, "Failed to unmarshal editor definition", "file", fileName)
				continue
			}

			if len(devfile) == 0 {
				continue
			}

			updateEditorDefinitionImages(devfile)
			editorContent, err = yaml.Marshal(devfile)
			if err != nil {
				return editorDefinitions, err
			}

			editorDefinitions[fileName] = editorContent
		}
	}

	return editorDefinitions, nil
}

func updateEditorDefinitionImages(devfile map[string]interface{}) {
	notAllowedCharsReg, _ := regexp.Compile("[^a-zA-Z0-9]+")

	metadata := devfile["metadata"].(map[string]interface{})
	devfileName := metadata["name"].(string)
	attributes := metadata["attributes"].(map[string]interface{})
	devfileVersion := fmt.Sprintf("%v", attributes["version"])

	components := devfile["components"].([]interface{})
	for _, component := range components {
		componentName := component.(map[string]interface{})["name"].(string)
		if container, ok := component.(map[string]interface{})["container"].(map[string]interface{}); ok {
			imageEnvName := fmt.Sprintf("%s_%s_%s_%s", editorDefinitionEnvNamePrefix, devfileName, devfileVersion, componentName)
			imageEnvName = notAllowedCharsReg.ReplaceAllString(imageEnvName, "_")
			imageEnvName = utils.GetArchitectureDependentEnvName(imageEnvName)

			if imageEnvValue, ok := os.LookupEnv(imageEnvName); ok {
				container["image"] = imageEnvValue
			}
		}
	}
}

func syncEditorDefinitions(cheCtx *chetypes.CheContext, editorDefinitions map[string][]byte) error {
	labels := chelabels.GetStandardLabels(componentName)

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        configMapName,
			Namespace:   cheCtx.CheCluster.Namespace,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}

	for fileName, content := range editorDefinitions {
		cm.Data[fileName] = string(content)
	}

	return resources.ConfigMap.Sync(cheCtx, cm)
}
