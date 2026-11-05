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

package imagepuller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"testing"

	"github.com/stretchr/testify/assert"
)

const testProjectCloneImage = "quay.io/devfile/project-clone:next"

func TestGetExternalImages(t *testing.T) {
	type testCase struct {
		name           string
		editorsFile    string
		samplesFile    string
		initObjects    []client.Object
		expectedImages []string
	}

	gatewayImage := defaults.GetGatewayImage(&chev2.CheCluster{})

	testCases := []testCase{
		{
			name:           "both editors and samples images",
			editorsFile:    "image-puller-resources-test/editors.json",
			samplesFile:    "image-puller-resources-test/samples.json",
			initObjects:    []client.Object{test.NewDWOPod("devworkspace-controller", testProjectCloneImage)},
			expectedImages: []string{"image_1", "image_2", "image_3", testProjectCloneImage, gatewayImage},
		},
		{
			name:           "no editors and samples images",
			editorsFile:    "image-puller-resources-test/empty.json",
			samplesFile:    "image-puller-resources-test/empty.json",
			initObjects:    []client.Object{test.NewDWOPod("devworkspace-controller", testProjectCloneImage)},
			expectedImages: []string{testProjectCloneImage, gatewayImage},
		},
		{
			name:           "no DevWorkspace Operator pod",
			editorsFile:    "image-puller-resources-test/empty.json",
			samplesFile:    "image-puller-resources-test/empty.json",
			expectedImages: []string{gatewayImage},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cheCtx := test.NewCtxBuilder().WithObjects(tc.initObjects...).Build()

			editorsEndpointUrl := getDashboardEditorsInternalAPIUrl(cheCtx)
			samplesEndpointUrl := getDashboardSamplesInternalAPIUrl(cheCtx)

			imagesProvider := &ExternalImagesProvider{
				imagesFilePath: filepath.Join(os.TempDir(), externalImagesStoreFileName),
				fetchRawDataFunc: func(url string) ([]byte, error) {
					switch url {
					case editorsEndpointUrl:
						return os.ReadFile(tc.editorsFile)
					case samplesEndpointUrl:
						return os.ReadFile(tc.samplesFile)
					case "sample_1_url":
						return os.ReadFile("image-puller-resources-test/sample_1.yaml")
					case "sample_2_url":
						return os.ReadFile("image-puller-resources-test/sample_2.yaml")
					default:
						return []byte{}, fmt.Errorf("unexpected url: %s", url)
					}
				},
			}

			images, err := imagesProvider.Get(cheCtx)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedImages, images)

			data, err := os.ReadFile(filepath.Join(os.TempDir(), externalImagesStoreFileName))
			assert.NoError(t, err)

			expectedFileContent := strings.Join(tc.expectedImages, "\n")
			assert.Equal(t, expectedFileContent, string(data))
		})
	}
}
