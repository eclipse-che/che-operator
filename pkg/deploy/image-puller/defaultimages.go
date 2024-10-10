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
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
)

// DefaultImagesProvider is an interface for fetching default images from a specific source.
type DefaultImagesProvider interface {
	get(namespace string) ([]string, error)
	persist(images []string, path string) error
}

type DashboardApiDefaultImagesProvider struct {
	DefaultImagesProvider
	// introduce in order to override in tests
	requestRawDataFunc func(url string) ([]byte, error)
}

func NewDashboardApiDefaultImagesProvider() *DashboardApiDefaultImagesProvider {
	return &DashboardApiDefaultImagesProvider{
		requestRawDataFunc: doRequestRawData,
	}
}

func (p *DashboardApiDefaultImagesProvider) get(namespace string) ([]string, error) {
	editorsEndpointUrl := fmt.Sprintf(
		"http://%s.%s.svc:8080/dashboard/api/editors",
		defaults.GetCheFlavor()+"-dashboard",
		namespace)

	editorsImages, err := p.readEditorImages(editorsEndpointUrl)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read default images: %w from endpoint %s", err, editorsEndpointUrl)
	}

	samplesEndpointUrl := fmt.Sprintf(
		"http://%s.%s.svc:8080/dashboard/api/airgap-sample",
		defaults.GetCheFlavor()+"-dashboard",
		namespace)

	samplesImages, err := p.readSampleImages(samplesEndpointUrl)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read default images: %w from endpoint %s", err, samplesEndpointUrl)
	}

	// using map to avoid duplicates
	allImages := make(map[string]bool)

	for _, image := range editorsImages {
		allImages[image] = true
	}
	for _, image := range samplesImages {
		allImages[image] = true
	}

	// having them sorted, prevents from constant changing CR spec
	return sortImages(allImages), nil
}

// readEditorImages reads list of images from editors:
// 1. reads list of devfile editors from the given endpoint (json objects array)
// 2. parses them and return images
func (p *DashboardApiDefaultImagesProvider) readEditorImages(entrypointUrl string) ([]string, error) {
	rawData, err := p.requestRawDataFunc(entrypointUrl)
	if err != nil {
		return []string{}, err
	}

	return parseEditorDevfiles(rawData)
}

// readSampleImages reads list of images from samples:
// 1. reads list of samples from the given endpoint (json objects array)
// 2. parses them and retrieves urls to a devfile
// 3. read and parses devfiles (yaml) and return images
func (p *DashboardApiDefaultImagesProvider) readSampleImages(entrypointUrl string) ([]string, error) {
	rawData, err := p.requestRawDataFunc(entrypointUrl)
	if err != nil {
		return []string{}, err
	}

	urls, err := parseSamples(rawData)
	if err != nil {
		return []string{}, err
	}

	allImages := make([]string, 0)
	for _, url := range urls {
		rawData, err = p.requestRawDataFunc(url)
		if err != nil {
			return []string{}, err
		}

		images, err := parseSampleDevfile(rawData)
		if err != nil {
			return []string{}, err
		}

		allImages = append(allImages, images...)
	}

	return allImages, nil
}

func (p *DashboardApiDefaultImagesProvider) persist(images []string, path string) error {
	return os.WriteFile(path, []byte(strings.Join(images, "\n")), 0644)
}

func sortImages(images map[string]bool) []string {
	sortedImages := make([]string, len(images))

	i := 0
	for image := range images {
		sortedImages[i] = image
		i++
	}

	sort.Strings(sortedImages)
	return sortedImages
}

func doRequestRawData(url string) ([]byte, error) {
	client := &http.Client{
		Transport: &http.Transport{},
		Timeout:   time.Second * 1,
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}

	rawData, err := io.ReadAll(response.Body)
	if err != nil {
		return []byte{}, err
	}

	_ = response.Body.Close()
	return rawData, nil
}

// parseSamples parse samples to collect urls to devfiles
func parseSamples(rawData []byte) ([]string, error) {
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

		if sample["url"] != nil {
			urls = append(urls, sample["url"].(string))
		}
	}

	return urls, nil
}

// parseDevfiles parse sample devfile represented as yaml to collect images
func parseSampleDevfile(rawData []byte) ([]string, error) {
	if len(rawData) == 0 {
		return []string{}, nil
	}

	var devfile map[string]interface{}
	if err := yaml.Unmarshal(rawData, &devfile); err != nil {
		return []string{}, err
	}

	return collectDevfileImages(devfile), nil
}

// parseEditorDevfiles parse editor devfiles represented as json array to collect images
func parseEditorDevfiles(rawData []byte) ([]string, error) {
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

		images = append(images, collectDevfileImages(devfile)...)
	}

	return images, nil
}

// collectDevfileImages retrieves images container component of the devfile.
func collectDevfileImages(devfile map[string]interface{}) []string {
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

		if container["image"] != nil {
			devfileImages = append(devfileImages, container["image"].(string))
		}
	}

	return devfileImages
}
