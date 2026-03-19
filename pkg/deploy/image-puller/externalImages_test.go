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

package imagepuller

import (
	"fmt"
	"os"

	"github.com/eclipse-che/che-operator/pkg/common/test"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetExternalImages(t *testing.T) {
	type testCase struct {
		name           string
		editorsFile    string
		samplesFile    string
		expectedImages []string
	}

	testCases := []testCase{
		{
			name:           "both editors and samples images",
			editorsFile:    "image-puller-resources-test/editors.json",
			samplesFile:    "image-puller-resources-test/samples.json",
			expectedImages: []string{"image_1", "image_2", "image_3"},
		},
		{
			name:           "no external images",
			editorsFile:    "image-puller-resources-test/empty.json",
			samplesFile:    "image-puller-resources-test/empty.json",
			expectedImages: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := test.NewCtxBuilder().Build()

			editorsEndpointUrl := getDashboardEditorsInternalAPIUrl(ctx)
			samplesEndpointUrl := getDashboardSamplesInternalAPIUrl(ctx)

			imagesProvider := &ExternalImagesProvider{
				imagesFilePath: externalImagesStoreFilePath,
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

			images, err := imagesProvider.Get(ctx)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedImages, images)

			data, err := os.ReadFile(externalImagesStoreFilePath)
			assert.NoError(t, err)

			expectedFileContent := ""
			for i, img := range tc.expectedImages {
				if i > 0 {
					expectedFileContent += "\n"
				}
				expectedFileContent += img
			}
			assert.Equal(t, expectedFileContent, string(data))
		})
	}
}
