//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package checlusterbackup

import (
	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
)

// CheckBackupSettings does reconcile on user provided backup settings.
// It does not do backup itself.
func (r *ReconcileCheClusterBackup) CheckBackupSettings(backupCR *orgv1.CheClusterBackup) (bool, error) {
	if backupCR.Spec.AutoconfigureRestBackupServer {
		// Use internal REST backup server
		err := r.EnsureDefaultBackupServerDeploymentExists(backupCR)
		if err != nil {
			return false, err
		}

		err = r.EnsureDefaultBackupServerServiceExists(backupCR)
		if err != nil {
			return false, err
		}

		err = r.EnsureInternalBackupServerConfigured(backupCR)
		if err != nil {
			return false, err
		}
	}

	done, err := r.ValidateBackupServerSettings(backupCR)
	if err != nil {
		return done, err
	}

	return true, nil
}
