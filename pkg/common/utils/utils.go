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

package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/yaml"
)

const (
	randomStringCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

var (
	delimiters = []string{",", "|", ";", ":", "#", "\t"}
)

// RandomString returns a cryptographically secure random string of the given length,
// consisting of uppercase and lowercase letters and digits.
func RandomString(length int) string {
	if length <= 0 {
		return ""
	}

	result := make([]byte, 0, length)
	buf := make([]byte, length)
	for len(result) < length {
		// crypto/rand.Read is guaranteed to never fail and to fill the whole buffer
		_, _ = rand.Read(buf)

		for _, b := range buf {
			if len(result) == length {
				break
			}

			// Rejection sampling to avoid modulo bias
			if int(b) < len(randomStringCharset)*(256/len(randomStringCharset)) {
				result = append(result, randomStringCharset[int(b)%len(randomStringCharset)])
			}
		}
	}

	return string(result)
}

func ValueOrDef[T comparable](value, defaultValue T) T {
	var zero T
	if value == zero {
		return defaultValue
	}
	return value
}

func DefaultIfEmpty[K comparable, V any](value, defaultValue map[K]V) map[K]V {
	if len(value) == 0 {
		return defaultValue
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
	data, err := os.ReadFile(yamlFile)
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

// InNamespaceEventFilter passes events for objects in the given namespace and for
// cluster-scoped objects (empty namespace). Watch mappers decide whether a
// cluster-scoped object is relevant.
func InNamespaceEventFilter(namespace string) predicate.Predicate {
	inScope := func(obj client.Object) bool {
		return obj.GetNamespace() == "" || obj.GetNamespace() == namespace
	}

	return predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return inScope(ce.Object)
		},
		DeleteFunc: func(de event.DeleteEvent) bool {
			return inScope(de.Object)
		},
		UpdateFunc: func(ue event.UpdateEvent) bool {
			return inScope(ue.ObjectOld)
		},
		GenericFunc: func(ge event.GenericEvent) bool {
			return inScope(ge.Object)
		},
	}
}

// FindAvailableDelimiter returns the first delimiter from a predefined list (comma, pipe, semicolon,
// colon, hash, tab) that is not present in the given string. This is useful for selecting a safe
// separator character when joining values that may themselves contain common delimiters.
// Returns an empty string if all candidate delimiters are already present in the input.
func FindAvailableDelimiter(s string) string {
	for _, delimiter := range delimiters {
		if !strings.Contains(s, delimiter) {
			return delimiter
		}
	}

	return ""
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

// FindProjectRoot walks the directory tree up until `go.mod` is found.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("failed to find project root: no go.mod found above the working directory")
		}

		dir = parent
	}
}
