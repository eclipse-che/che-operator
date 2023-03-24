//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"
)

func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func Remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func GeneratePassword(stringLength int) (passwd string) {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := stringLength
	buf := make([]rune, length)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	passwd = string(buf)
	return passwd
}

func IsK8SResourceServed(discoveryClient discovery.DiscoveryInterface, resourceName string) bool {
	_, resourceList, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false
	}

	return hasAPIResourceNameInList(resourceName, resourceList)
}

func hasAPIResourceNameInList(name string, resources []*metav1.APIResourceList) bool {
	for _, l := range resources {
		for _, r := range l.APIResources {
			if r.Name == name {
				return true
			}
		}
	}

	return false
}

func GetValue(value string, defaultValue string) string {
	if value == "" {
		value = defaultValue
	}
	return value
}

// GetImageNameAndTag returns the image repository and tag name from the provided image
// Referenced from https://github.com/che-incubator/chectl/blob/main/src/util.ts
func GetImageNameAndTag(image string) (string, string) {
	var imageName, imageTag string
	if strings.Contains(image, "@") {
		// Image is referenced via a digest
		index := strings.Index(image, "@")
		imageName = image[:index]
		imageTag = image[index+1:]
	} else {
		// Image is referenced via a tag
		lastColonIndex := strings.LastIndex(image, ":")
		if lastColonIndex == -1 {
			imageName = image
			imageTag = "latest"
		} else {
			beforeLastColon := image[:lastColonIndex]
			afterLastColon := image[lastColonIndex+1:]
			if strings.Contains(afterLastColon, "/") {
				// The colon is for registry port and not for a tag
				imageName = image
				imageTag = "latest"
			} else {
				// The colon separates image name from the tag
				imageName = beforeLastColon
				imageTag = afterLastColon
			}
		}
	}
	return imageName, imageTag
}

func ReadObjectInto(yamlFile string, obj interface{}) error {
	data, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, obj)
	if err != nil {
		return err
	}

	return nil
}

func ComputeHash256(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func GetPullPolicyFromDockerImage(dockerImage string) string {
	tag := ""
	parts := strings.Split(dockerImage, ":")
	if len(parts) > 1 {
		tag = parts[1]
	}
	if tag == "latest" || tag == "nightly" || tag == "next" {
		return "Always"
	}
	return "IfNotPresent"
}

func GetMap(value map[string]string, defaultValue map[string]string) map[string]string {
	ret := value
	if len(value) < 1 {
		ret = defaultValue
	}

	return ret
}

func InNamespaceEventFilter(namespace string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return namespace == ce.Object.GetNamespace()
		},
		DeleteFunc: func(de event.DeleteEvent) bool {
			return namespace == de.Object.GetNamespace()
		},
		UpdateFunc: func(ue event.UpdateEvent) bool {
			return namespace == ue.ObjectOld.GetNamespace()
		},
		GenericFunc: func(ge event.GenericEvent) bool {
			return namespace == ge.Object.GetNamespace()
		},
	}
}

func ParseMap(src string) map[string]string {
	if src == "" {
		return nil
	}

	m := map[string]string{}
	for _, item := range strings.Split(src, ",") {
		keyValuePair := strings.Split(item, "=")
		if len(keyValuePair) == 1 {
			continue
		}

		key := strings.TrimSpace(keyValuePair[0])
		value := strings.TrimSpace(keyValuePair[1])
		if key != "" && value != "" {
			m[keyValuePair[0]] = keyValuePair[1]
		}
	}

	return m
}

func CloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Converts label map into plain string
func FormatLabels(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}

	return labels.FormatLabels(m)
}

// Whitelists the host.
// Sample: Whitelist("che.yourcompany.com") -> ".yourcompany.com"
func Whitelist(hostname string) (value string) {
	i := strings.Index(hostname, ".")
	if i > -1 {
		j := strings.LastIndex(hostname, ".")
		if j > i {
			return hostname[i:]
		}
	}
	return hostname
}
