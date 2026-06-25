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

package openvsx_database

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *OpenVSXDatabaseReconciler) syncUserSetupJob(ctx *chetypes.DeployContext) (bool, error) {
	existing := &batchv1.Job{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      constants.OpenVSXDatabaseSetupJobName,
		Namespace: ctx.CheCluster.Namespace,
	}, existing)

	if err == nil {
		return true, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	image := defaults.GetOpenVSXDatabaseImage(ctx.CheCluster)
	pullPolicy := utils.GetPullPolicyFromDockerImage(image)

	labels := deploy.GetLabels(constants.OpenVSXDatabaseComponentName)

	//backoffLimit := int32(3)
	//parallelism := int32(1)
	//completions := int32(1)
	//terminationGracePeriodSeconds := int64(30)
	//runAsNonRoot := true

	secretName := openvsx.GetCredentialsSecretName(ctx)

	dbEnvVars := []corev1.EnvVar{
		{
			Name:  "PGHOST",
			Value: constants.OpenVSXDatabaseComponentName,
		},
		envFromSecret("PGDATABASE", secretName, "database-name"),
		envFromSecret("PGUSER", secretName, "database-user"),
		envFromSecret("PGPASSWORD", secretName, "database-password"),
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			//Name:      openvsx_server.userSetupJobName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-schema",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env:             dbEnvVars,
							Command: []string{"sh", "-c",
								`until psql -c "SELECT 1 FROM user_data LIMIT 0" 2>/dev/null; do echo "Waiting for Flyway migrations..."; sleep 5; done`,
							},
						},
					},
					Containers: []corev1.Container{
						{
							//Name:            openvsx_server.userSetupJobName,
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env: append(dbEnvVars,
								envFromSecret("OPENVSX_USER_NAME", secretName, "publisher-name"),
								envFromSecret("OPENVSX_USER_PAT", secretName, "publisher-token"),
								envFromSecret("OPENVSX_ADMIN_NAME", secretName, "admin-name"),
								envFromSecret("OPENVSX_ADMIN_PAT", secretName, "admin-token"),
							),
							Command: []string{"sh", "-c", `
psql \
  -v user_name="$OPENVSX_USER_NAME" \
  -v user_pat="$OPENVSX_USER_PAT" \
  -v admin_name="$OPENVSX_ADMIN_NAME" \
  -v admin_pat="$OPENVSX_ADMIN_PAT" \
  <<'EOSQL'
INSERT INTO user_data (id, login_name, role)
  VALUES (1001, :'user_name', 'privileged')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO personal_access_token
  (id, user_data, value, active, created_timestamp, accessed_timestamp, description, notified)
  VALUES (1001, 1001, :'user_pat', true, current_timestamp, current_timestamp, 'extensions publisher', false)
  ON CONFLICT (id) DO NOTHING;

INSERT INTO user_data (id, login_name, role)
  VALUES (1002, :'admin_name', 'admin')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO personal_access_token
  (id, user_data, value, active, created_timestamp, accessed_timestamp, description, notified)
  VALUES (1002, 1002, :'admin_pat', true, current_timestamp, current_timestamp, 'Admin API Token', false)
  ON CONFLICT (id) DO NOTHING;
EOSQL`,
							},
						},
					},
				},
			},
			Parallelism:  &parallelism,
			BackoffLimit: &backoffLimit,
			Completions:  &completions,
		},
	}

	return deploy.Sync(ctx, job, deploy.JobDiffOpts)
}
