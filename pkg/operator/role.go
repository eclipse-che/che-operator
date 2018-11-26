package operator

import (
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

func newRole(name string) *rbac.Role {
	labels := map[string]string{"app": "che"}
	return &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			Namespace: namespace,
			Labels:    labels,

		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods/exec",
				},
				Verbs: []string{
					"create",
				},

			},
		},
	}
}

func CreateNewRole(name string) (*rbac.Role) {
	role := newRole(name)
	if err := sdk.Create(role); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+name+" role : %v", err)
		return nil
	}
	return role
}