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

package pluginregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var (
	editorsDefinitionsDir           = "/tmp/editors-definitions"
	editorsDefinitionsConfigMapName = "editors-definitions"
)

func (p *PluginRegistryReconciler) syncEditors(ctx *chetypes.DeployContext) (bool, error) {
	editorDefinitions, err := readEditorDefinitions()
	if err != nil {
		return false, err
	}

	done, err := syncEditorDefinitions(ctx, editorDefinitions)
	if !done {
		return false, err
	}

	return true, nil
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
				return editorDefinitions, err
			}

			updateEditorDefinitionImageFromEnv(devfile)

			editorContent, err = yaml.Marshal(devfile)
			if err != nil {
				return editorDefinitions, err
			}

			editorDefinitions[fileName] = editorContent
		}
	}

	return editorDefinitions, nil
}

func updateEditorDefinitionImageFromEnv(devfile map[string]interface{}) {
	notAllowedCharsReg, _ := regexp.Compile("[^a-zA-Z0-9]+")

	metadata := devfile["metadata"].(map[string]interface{})
	devfileName := metadata["name"].(string)
	attributes := metadata["attributes"].(map[string]interface{})
	devfileVersion := attributes["version"].(string)

	components := devfile["components"].([]interface{})
	for _, component := range components {
		componentName := component.(map[string]interface{})["name"].(string)
		if container, ok := component.(map[string]interface{})["container"].(map[string]interface{}); ok {
			imageEnvName := fmt.Sprintf("RELATED_IMAGE_%s_%s_%s", devfileName, devfileVersion, componentName)
			imageEnvName = notAllowedCharsReg.ReplaceAllString(imageEnvName, "_")
			imageEnvName = utils.GetArchitectureDependentEnvName(imageEnvName)

			if imageEnvValue, ok := os.LookupEnv(imageEnvName); ok {
				container["image"] = imageEnvValue
			}
		}
	}
}

func syncEditorDefinitions(ctx *chetypes.DeployContext, editorDefinitions map[string][]byte) (bool, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        editorsDefinitionsConfigMapName,
			Namespace:   ctx.CheCluster.Namespace,
			Labels:      deploy.GetLabels(constants.EditorDefinitionComponentName),
			Annotations: map[string]string{},
		},
		Data: map[string]string{},
	}

	for fileName, content := range editorDefinitions {
		cm.Data[fileName] = string(content)
	}

	return deploy.Sync(ctx, cm, deploy.ConfigMapDiffOpts)
}
