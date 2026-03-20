//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"sigs.k8s.io/yaml"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
)

const externalImagesStoreFileName = "external_images.txt"

type ExternalImagesProvider struct {
	// Path to store retrieved external images
	imagesFilePath string

	// exposed for testing purpose only, func to get content by url
	fetchRawDataFunc func(url string) ([]byte, error)
}

func NewExternalImagesProvider() *ExternalImagesProvider {
	p := &ExternalImagesProvider{
		imagesFilePath:   filepath.Join(os.TempDir(), externalImagesStoreFileName),
		fetchRawDataFunc: fetchRawData,
	}
	return p
}

func (p *ExternalImagesProvider) Get(ctx *chetypes.DeployContext) ([]string, error) {
	images, err := p.read(ctx)
	if err != nil {
		return []string{}, err
	}

	err = p.write(images)
	if err != nil {
		return []string{}, err
	}

	return images, nil
}

func (p *ExternalImagesProvider) read(ctx *chetypes.DeployContext) ([]string, error) {
	editorsImages, err := p.fetchEditorImages(ctx)
	if err != nil {
		return []string{}, err
	}

	samplesImages, err := p.fetchSampleImages(ctx)
	if err != nil {
		return []string{}, err
	}

	var images []string
	images = append(images, editorsImages...)
	images = append(images, samplesImages...)
	sort.Strings(images)
	images = slices.Compact(images)

	return images, nil
}

func (p *ExternalImagesProvider) write(images []string) error {
	return os.WriteFile(p.imagesFilePath, []byte(strings.Join(images, "\n")), 0644)
}

// fetchSampleImages fetches list of images from samples:
// 1. reads list of samples from the given endpoint (json objects array)
// 2. parses them and retrieves urls to a devfile
// 3. read and parses devfiles (yaml) and return images
func (p *ExternalImagesProvider) fetchSampleImages(ctx *chetypes.DeployContext) ([]string, error) {
	url := getDashboardSamplesInternalAPIUrl(ctx)

	rawData, err := p.fetchRawDataFunc(url)
	if err != nil {
		return []string{}, err
	}

	urls, err := p.parseSampleURLs(rawData)
	if err != nil {
		return []string{}, err
	}

	sampleImages := make([]string, 0)
	for _, url := range urls {
		rawData, err = p.fetchRawDataFunc(url)
		if err != nil {
			return []string{}, err
		}

		images, err := p.parseSampleDevfile(rawData)
		if err != nil {
			return []string{}, err
		}

		sampleImages = append(sampleImages, images...)
	}

	return sampleImages, nil
}

// parseSampleURLs parses samples to collect urls to devfiles
func (p *ExternalImagesProvider) parseSampleURLs(rawData []byte) ([]string, error) {
	if len(rawData) == 0 {
		return []string{}, nil
	}

	var samples []interface{}
	if err := json.Unmarshal(rawData, &samples); err != nil {
		return []string{}, err
	}

	urls := make([]string, 0)

	for i := range samples {
		sample, ok := samples[i].(map[string]interface{})
		if !ok {
			continue
		}

		url, ok := sample["url"].(string)
		if ok {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

// parseDevfiles parse sample devfile represented as yaml to collect images
func (p *ExternalImagesProvider) parseSampleDevfile(rawData []byte) ([]string, error) {
	var devfile map[string]interface{}
	if err := yaml.Unmarshal(rawData, &devfile); err != nil {
		return []string{}, err
	}

	return p.extractContainerImages(devfile), nil
}

// fetchEditorImages fetches list of images from editors:
// 1. reads list of devfile editors from the given endpoint (json objects array)
// 2. parses them and return images
func (p *ExternalImagesProvider) fetchEditorImages(ctx *chetypes.DeployContext) ([]string, error) {
	url := getDashboardEditorsInternalAPIUrl(ctx)

	rawData, err := p.fetchRawDataFunc(url)
	if err != nil {
		return []string{}, err
	}

	images, err := p.parseEditor(rawData)
	if err != nil {
		return []string{}, err
	}

	return images, nil
}

// parseEditor parse editor devfiles represented as json array to collect images
func (p *ExternalImagesProvider) parseEditor(rawData []byte) ([]string, error) {
	if len(rawData) == 0 {
		return []string{}, nil
	}

	var devfiles []interface{}
	if err := json.Unmarshal(rawData, &devfiles); err != nil {
		return []string{}, err
	}

	images := make([]string, 0)

	for i := range devfiles {
		devfile, ok := devfiles[i].(map[string]interface{})
		if !ok {
			continue
		}

		images = append(images, p.extractContainerImages(devfile)...)
	}

	return images, nil
}

// extractContainerImages retrieves images from container components of the devfile.
func (p *ExternalImagesProvider) extractContainerImages(devfile map[string]interface{}) []string {
	devfileImages := make([]string, 0)

	components, ok := devfile["components"].([]interface{})
	if !ok {
		return []string{}
	}

	for k := range components {
		component, ok := components[k].(map[string]interface{})
		if !ok {
			continue
		}

		container, ok := component["container"].(map[string]interface{})
		if !ok {
			continue
		}

		if image, ok := container["image"].(string); ok {
			devfileImages = append(devfileImages, image)
		}
	}

	return devfileImages
}

func getDashboardBaseInternalURL(ctx *chetypes.DeployContext) string {
	namespace := ctx.CheCluster.Namespace
	serviceName := defaults.GetCheFlavor() + "-dashboard"
	return fmt.Sprintf("http://%s.%s.svc:8080/dashboard/api", serviceName, namespace)
}

func getDashboardEditorsInternalAPIUrl(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf("%s/editors", getDashboardBaseInternalURL(ctx))
}

func getDashboardSamplesInternalAPIUrl(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf("%s/airgap-sample", getDashboardBaseInternalURL(ctx))
}

func fetchRawData(url string) ([]byte, error) {
	client := &http.Client{
		Transport: &http.Transport{},
		Timeout:   time.Second * 5,
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			logger.Error(err, "unable to close response body")
		}
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return []byte{}, fmt.Errorf("unexpected status code %d for URL %s", response.StatusCode, url)
	}

	rawData, err := io.ReadAll(response.Body)
	if err != nil {
		return []byte{}, err
	}

	return rawData, nil
}
