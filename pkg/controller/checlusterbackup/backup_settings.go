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

// CheckBackupSettings does reconcile on user provided backup settings.
// It does not do backup itself.
func CheckBackupSettings(bctx *BackupContext) (bool, error) {
	if bctx.backupCR.Spec.AutoconfigureRestBackupServer {
		// Use internal REST backup server
		done, err := ConfigureInternalBackupServer(bctx)
		if err != nil || !done {
			return done, err
		}
	}

	// Check if current backup server is configured properly
	done, err := bctx.backupServer.ValidateConfiguration(bctx)
	if err != nil {
		return done, err
	}

	return true, nil
}
