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

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadEditorImages(t *testing.T) {
	imagesProvider := &DashboardApiDefaultImagesProvider{
		requestRawDataFunc: func(url string) ([]byte, error) {
			return os.ReadFile("image-puller-resources-test/editors.json")
		},
	}

	images, err := imagesProvider.readEditorImages("")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(images))
	assert.Contains(t, images, "image_1")
	assert.Contains(t, images, "image_2")
}

func TestSampleImages(t *testing.T) {
	imagesProvider := &DashboardApiDefaultImagesProvider{
		requestRawDataFunc: func(url string) ([]byte, error) {
			switch url {
			case "":
				return os.ReadFile("image-puller-resources-test/samples.json")
			case "sample_1_url":
				return os.ReadFile("image-puller-resources-test/sample_1.yaml")
			case "sample_2_url":
				return os.ReadFile("image-puller-resources-test/sample_2.yaml")
			default:
				return []byte{}, fmt.Errorf("unexpected url: %s", url)
			}
		},
	}

	images, err := imagesProvider.readSampleImages("")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(images))
	assert.Contains(t, images, "image_1")
	assert.Contains(t, images, "image_3")
}

func TestGet(t *testing.T) {
	imagesProvider := &DashboardApiDefaultImagesProvider{
		requestRawDataFunc: func(url string) ([]byte, error) {
			samplesEndpointUrl := fmt.Sprintf(
				"http://%s.eclipse-che.svc:8080/dashboard/api/airgap-sample",
				defaults.GetCheFlavor()+"-dashboard")
			editorsEndpointUrl := fmt.Sprintf(
				"http://%s.eclipse-che.svc:8080/dashboard/api/editors",
				defaults.GetCheFlavor()+"-dashboard")

			switch url {
			case editorsEndpointUrl:
				return os.ReadFile("image-puller-resources-test/editors.json")
			case samplesEndpointUrl:
				return os.ReadFile("image-puller-resources-test/samples.json")
			case "sample_1_url":
				return os.ReadFile("image-puller-resources-test/sample_1.yaml")
			case "sample_2_url":
				return os.ReadFile("image-puller-resources-test/sample_2.yaml")
			default:
				return []byte{}, fmt.Errorf("unexpected url: %s", url)
			}
		},
	}

	images, err := imagesProvider.get("eclipse-che")
	assert.NoError(t, err)
	assert.Equal(t, 3, len(images))
	assert.Equal(t, "image_1", images[0])
	assert.Equal(t, "image_2", images[1])
	assert.Equal(t, "image_3", images[2])

	err = imagesProvider.persist(images, "/tmp/images.txt")
	assert.NoError(t, err)

	data, err := os.ReadFile("/tmp/images.txt")
	assert.NoError(t, err)

	assert.Equal(t, "image_1\nimage_2\nimage_3", string(data))
}
