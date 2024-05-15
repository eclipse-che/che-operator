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
	"os"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func TestReadEditorDefinitions(t *testing.T) {
	err := os.Setenv("RELATED_IMAGE_che_code_1_2_3_component_a", "image-new-a")
	assert.NoError(t, err)

	defer func() {
		_ = os.Setenv("RELATED_IMAGE_che_code_1_2_3_component_a", "")
	}()

	editorDefinitions, err := readEditorDefinitions()
	assert.NoError(t, err)
	assert.NotEmpty(t, editorDefinitions)
	assert.Equal(t, 1, len(editorDefinitions))
	assert.Contains(t, editorDefinitions, "devfile.yaml")

	var devfile map[string]interface{}
	err = yaml.Unmarshal(editorDefinitions["devfile.yaml"], &devfile)
	assert.NoError(t, err)

	components := devfile["components"].([]interface{})

	component := components[0].(map[string]interface{})
	container := component["container"].(map[string]interface{})
	assert.Equal(t, "image-new-a", container["image"])

	component = components[1].(map[string]interface{})
	container = component["container"].(map[string]interface{})
	assert.Equal(t, "image-b", container["image"])

	component = components[2].(map[string]interface{})
	container, ok := component["container"].(map[string]interface{})
	assert.False(t, ok)
}

func TestSyncEditorDefinitions(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	editorDefinitions, err := readEditorDefinitions()
	assert.NoError(t, err)
	assert.NotEmpty(t, editorDefinitions)

	done, err := syncEditorDefinitions(ctx, editorDefinitions)
	assert.NoError(t, err)
	assert.True(t, done)
}
