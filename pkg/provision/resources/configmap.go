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

package resources

import (
	"slices"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ConfigMap = &configMapHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	},
}

type configMapHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h *configMapHandler) Sync(cheCtx *chetypes.CheContext, cm *corev1.ConfigMap) error {
	return syncher.SyncObject(
		cheCtx,
		cm,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

// List returns the config maps in the given namespace matching the labels
// (optional) and annotations (optional). A config map matches when all the given
// labels and annotations are present with exactly the given values; annotations
// are filtered in memory, since Kubernetes list options only support label selectors.
func (h *configMapHandler) List(
	cheCtx *chetypes.CheContext,
	namespace string,
	matchLabels map[string]string,
	matchAnnotations map[string]string,
) ([]corev1.ConfigMap, error) {
	opts := &client.ListOptions{Namespace: namespace}
	if len(matchLabels) > 0 {
		opts.LabelSelector = labels.SelectorFromSet(matchLabels)
	}

	configMaps := &corev1.ConfigMapList{}
	if err := syncher.List(cheCtx, configMaps, opts); err != nil {
		return nil, err
	}

	matched := slices.DeleteFunc(
		configMaps.Items,
		func(configMap corev1.ConfigMap) bool {
			return !utils.ContainsEntries(configMap.Annotations, matchAnnotations)
		})

	return matched, nil
}
