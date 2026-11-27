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

var Secret = &secretHandler{
	suppressDiff:       true,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta"),
	},
}

type secretHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h *secretHandler) Sync(cheCtx *chetypes.CheContext, secret *corev1.Secret) error {
	return syncher.SyncObject(
		cheCtx,
		secret,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h *secretHandler) Get(cheCtx *chetypes.CheContext, name string, namespace string, secret *corev1.Secret) (bool, error) {
	return syncher.GetObject(
		cheCtx,
		name,
		namespace,
		secret,
	)
}

func (h *secretHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return syncher.Delete(cheCtx, name, namespace, &corev1.Secret{})
}

// List returns the secrets in the given namespace matching the labels
// (optional) and annotations (optional). A secret matches when all the given
// labels and annotations are present with exactly the given values; annotations
// are filtered in memory, since Kubernetes list options only support label selectors.
func (h *secretHandler) List(
	cheCtx *chetypes.CheContext,
	namespace string,
	matchLabels map[string]string,
	matchAnnotations map[string]string,
) ([]corev1.Secret, error) {
	listOptions := &client.ListOptions{Namespace: namespace}
	if len(matchLabels) > 0 {
		listOptions.LabelSelector = labels.SelectorFromSet(matchLabels)
	}

	secrets := &corev1.SecretList{}
	if err := syncher.List(cheCtx, secrets, listOptions); err != nil {
		return nil, err
	}

	matched := slices.DeleteFunc(
		secrets.Items,
		func(secret corev1.Secret) bool {
			return !utils.ContainsEntries(secret.Annotations, matchAnnotations)
		})

	return matched, nil
}
