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

package editorsdefinitions

import (
	"os"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestReadEditorDefinitions(t *testing.T) {
	err := os.Setenv("RELATED_IMAGE_editor_definition_che_code_1_2_3_component_a", "image-new-1_2_3-a")
	err = os.Setenv("RELATED_IMAGE_editor_definition_che_code_2022_1_component_a", "image-new-2022_1-a")
	assert.NoError(t, err)

	defer func() {
		_ = os.Setenv("RELATED_IMAGE_editor_definition_che_code_1_2_3_component_a", "")
		_ = os.Setenv("RELATED_IMAGE_editor_definition_che_code_2022_1_component_a", "")
	}()

	editorDefinitions, err := readEditorDefinitions()
	assert.NoError(t, err)
	assert.NotEmpty(t, editorDefinitions)
	assert.Equal(t, 2, len(editorDefinitions))
	assert.Contains(t, editorDefinitions, "devfile-1.yaml")
	assert.Contains(t, editorDefinitions, "devfile-2.yaml")

	var devfile map[string]interface{}
	err = yaml.Unmarshal(editorDefinitions["devfile-1.yaml"], &devfile)
	assert.NoError(t, err)

	components := devfile["components"].([]interface{})
	component := components[0].(map[string]interface{})
	container := component["container"].(map[string]interface{})
	assert.Equal(t, "image-new-1_2_3-a", container["image"])

	component = components[1].(map[string]interface{})
	container = component["container"].(map[string]interface{})
	assert.Equal(t, "image-b", container["image"])

	component = components[2].(map[string]interface{})
	container, ok := component["container"].(map[string]interface{})
	assert.False(t, ok)

	err = yaml.Unmarshal(editorDefinitions["devfile-2.yaml"], &devfile)
	assert.NoError(t, err)

	components = devfile["components"].([]interface{})
	component = components[0].(map[string]interface{})
	container = component["container"].(map[string]interface{})
	assert.Equal(t, "image-new-2022_1-a", container["image"])
}

func TestSyncEditorDefinitions(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	editorDefinitions, err := readEditorDefinitions()
	assert.NoError(t, err)
	assert.NotEmpty(t, editorDefinitions)
	assert.Len(t, editorDefinitions, 2)

	done, err := syncEditorDefinitions(ctx, editorDefinitions)
	assert.NoError(t, err)
	assert.True(t, done)
}
