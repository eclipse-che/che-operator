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
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *OpenVSXDatabaseReconciler) syncDatabaseProvisioned(ctx *chetypes.DeployContext) (bool, error) {
	image := defaults.GetOpenVSXDatabaseImage(ctx.CheCluster)
	imagePullPolicy := utils.GetPullPolicyFromDockerImage(image)

	labels := deploy.GetLabels(constants.OpenVSXDatabaseComponentName)

	secretName := openvsx.GetCredentialsSecretName(ctx)

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXDatabaseProvisionJobName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:            constants.OpenVSXDatabaseProvisionJobName + "-init",
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							Env: []corev1.EnvVar{
								{
									Name:  "PGHOST",
									Value: constants.OpenVSXDatabaseComponentName,
								},
								utils.EnvVarFromSecret("PGDATABASE", secretName, "database-name"),
								utils.EnvVarFromSecret("PGUSER", secretName, "database-user"),
								utils.EnvVarFromSecret("PGPASSWORD", secretName, "database-password"),
							},
							Command: []string{"sh", "-c",
								`until psql -c "SELECT 1 FROM user_data LIMIT 0" 2>/dev/null; do echo "Waiting for Flyway migrations..."; sleep 5; done`,
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            constants.OpenVSXDatabaseProvisionJobName,
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							Env: []corev1.EnvVar{
								{
									Name:  "PGHOST",
									Value: constants.OpenVSXDatabaseComponentName,
								},
								utils.EnvVarFromSecret("PGDATABASE", secretName, "database-name"),
								utils.EnvVarFromSecret("PGUSER", secretName, "database-user"),
								utils.EnvVarFromSecret("PGPASSWORD", secretName, "database-password"),
								utils.EnvVarFromSecret("OPENVSX_USER_NAME", secretName, "openvsx-publisher-name"),
								utils.EnvVarFromSecret("OPENVSX_USER_PAT", secretName, "openvsx-publisher-token"),
								utils.EnvVarFromSecret("OPENVSX_ADMIN_NAME", secretName, "openvsx-admin-name"),
								utils.EnvVarFromSecret("OPENVSX_ADMIN_PAT", secretName, "openvsx-admin-token"),
							},
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
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: ptr.To(int64(30)),
				},
			},
			Parallelism:             ptr.To(int32(1)),
			BackoffLimit:            ptr.To(int32(3)),
			Completions:             ptr.To(int32(1)),
			TTLSecondsAfterFinished: ptr.To(int32(60)),
			ActiveDeadlineSeconds:   ptr.To(int64(300)),
		},
	}

	deploy.EnsurePodSecurityStandards(
		&job.Spec.Template.Spec,
		constants.DefaultSecurityContextRunAsUser,
		constants.DefaultSecurityContextFsGroup,
	)

	if err := controllerutil.SetControllerReference(ctx.CheCluster, job, ctx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		job,
		&k8sclient.SyncOptions{
			DeleteOpts: []client.DeleteOption{client.PropagationPolicy(metav1.DeletePropagationBackground)},
		},
	)
	if errors.IsAlreadyExists(err) {
		return false, nil
	}

	return err == nil, err
}
