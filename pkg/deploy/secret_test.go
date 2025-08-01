//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	"context"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetSecrets(t *testing.T) {
	type testCase struct {
		name           string
		labels         map[string]string
		annotations    map[string]string
		initObjects    []client.Object
		expectedAmount int
	}

	runtimeSecrets := []client.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"l1": "v1",
					"l2": "v2",
				},
				Annotations: map[string]string{
					"a1": "v1",
					"a2": "v2",
				},
			},
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test1",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"l1": "v1",
					"l3": "v3",
				},
				Annotations: map[string]string{
					"a1": "v1",
					"a3": "v3",
				},
			},
		},
	}

	testCases := []testCase{
		{
			name:        "Get secrets",
			initObjects: []client.Object{},
			labels: map[string]string{
				"l1": "v1",
			},
			annotations: map[string]string{
				"a1": "v1",
			},
			expectedAmount: 2,
		},
		{
			name:        "Get secrets",
			initObjects: []client.Object{},
			labels: map[string]string{
				"l1": "v1",
			},
			annotations: map[string]string{
				"a1": "v1",
				"a2": "v2",
			},
			expectedAmount: 1,
		},
		{
			name:        "Get secrets",
			initObjects: []client.Object{},
			labels: map[string]string{
				"l1": "v1",
				"l2": "v2",
			},
			annotations: map[string]string{
				"a1": "v1",
			},
			expectedAmount: 1,
		},
		{
			name:        "Get secrets, unknown label",
			initObjects: []client.Object{},
			labels: map[string]string{
				"l4": "v4",
			},
			annotations:    map[string]string{},
			expectedAmount: 0,
		},
		{
			name:        "Get secrets, unknown annotation",
			initObjects: []client.Object{},
			labels: map[string]string{
				"l1": "v1",
			},
			annotations: map[string]string{
				"a4": "v4",
			},
			expectedAmount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.initObjects = append(testCase.initObjects, runtimeSecrets...)
			ctx := test.NewCtxBuilder().WithObjects(testCase.initObjects...).Build()

			secrets, err := GetSecrets(ctx, testCase.labels, testCase.annotations)
			if err != nil {
				t.Fatalf("Error getting secrets: %v", err)
			}

			if len(secrets) != testCase.expectedAmount {
				t.Fatalf("Expected %d but found: %d", testCase.expectedAmount, len(secrets))
			}
		})
	}
}

func TestSyncSecretToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncSecretToCluster(ctx, "test", "eclipse-che", map[string][]byte{"A": []byte("AAAA")})
	if !done || err != nil {
		t.Fatalf("Failed to sync secret: %v", err)
	}

	// sync another secret
	done, err = SyncSecretToCluster(ctx, "test", "eclipse-che", map[string][]byte{"B": []byte("BBBB")})
	if !done || err != nil {
		t.Fatalf("Failed to sync secret: %v", err)
	}

	actual := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}

	if len(actual.Data) != 1 {
		t.Fatalf("Failed to sync secret: %v", err)
	}
	if string(actual.Data["B"]) != "BBBB" {
		t.Fatalf("Failed to sync secret: %v", err)
	}
}
