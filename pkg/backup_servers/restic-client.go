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
package backup_servers

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	resticCli = "restic"

	resticPasswordCommandEnvVarName = "RESTIC_PASSWORD_COMMAND"
)

type ResticClient struct {
	RepoUrl       string
	RepoPassword  string
	AdditionalEnv []string
}

type SnapshotStat struct {
	Id   string
	Info string
}

func (c *ResticClient) InitRepository() (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.RepoPassword)

	initCommand := exec.Command(resticCli, "--repo", c.RepoUrl, "init")
	initCommand.Env = os.Environ()
	initCommand.Env = append(initCommand.Env, resticPasswordCommandEnvVar)
	if c.AdditionalEnv != nil {
		initCommand.Env = append(initCommand.Env, c.AdditionalEnv...)
	}

	if output, err := initCommand.CombinedOutput(); err != nil {
		logrus.Error(string(output))
		return true, err
	}

	return true, nil
}

func (c *ResticClient) IsRepositoryExist() (bool, bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.RepoPassword)

	snapshotsCommand := exec.Command(resticCli, "--repo", c.RepoUrl, "snapshots")
	snapshotsCommand.Env = os.Environ()
	snapshotsCommand.Env = append(snapshotsCommand.Env, resticPasswordCommandEnvVar)
	if c.AdditionalEnv != nil {
		snapshotsCommand.Env = append(snapshotsCommand.Env, c.AdditionalEnv...)
	}

	// if output, err := snapshotsCommand.CombinedOutput(); err != nil {
	output, err := snapshotsCommand.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "Is there a repository at the following location?") {
			return false, true, nil
		}
		logrus.Error(string(output))
		return false, true, err
	}

	return true, true, nil
}

func (c *ResticClient) CheckRepository() (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.RepoPassword)

	checkCommand := exec.Command(resticCli, "--repo", c.RepoUrl, "check")
	checkCommand.Env = os.Environ()
	checkCommand.Env = append(checkCommand.Env, resticPasswordCommandEnvVar)
	if c.AdditionalEnv != nil {
		checkCommand.Env = append(checkCommand.Env, c.AdditionalEnv...)
	}

	if output, err := checkCommand.CombinedOutput(); err != nil {
		logrus.Error(string(output))
		return true, err
	}

	return true, nil
}

func (c *ResticClient) SendSnapshot(path string) (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.RepoPassword)

	// Check that there is data to send
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true, err
	}

	backupCommand := exec.Command(resticCli, "--repo", c.RepoUrl, "backup", ".")
	// Change directory to the backup root to avoid backup root path in the target folder after restore
	backupCommand.Dir = path
	backupCommand.Env = os.Environ()
	backupCommand.Env = append(backupCommand.Env, resticPasswordCommandEnvVar)
	if c.AdditionalEnv != nil {
		backupCommand.Env = append(backupCommand.Env, c.AdditionalEnv...)
	}

	out, err := backupCommand.CombinedOutput()
	output := string(out)
	if err != nil {
		logrus.Error(output)
		return true, err
	}

	stat := SnapshotStat{}
	snapshotIdRegex := regexp.MustCompile("snapshot ([0-9a-f]+) saved")
	snapshotIdMatch := snapshotIdRegex.FindStringSubmatch(output)
	if len(snapshotIdMatch) > 0 {
		stat.Id = snapshotIdMatch[1]
	}
	snapshotSizeRegex := regexp.MustCompile("processed (.*)")
	snapshotSizeMatch := snapshotSizeRegex.FindStringSubmatch(output)
	if len(snapshotSizeMatch) > 0 {
		stat.Info = snapshotSizeMatch[1]
	}
	// Log the fact of successful sending of a snapshot
	logrus.Infof("Snapshot %s uploaded: %s", stat.Id, stat.Info)

	return true, nil
}

func (c *ResticClient) DownloadSnapshot(snapshot string, path string) (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.RepoPassword)

	// Ensure destination path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return true, fmt.Errorf("failed to create directory for snapshot. Reason: %s", err.Error())
		}
	}

	restoreCommand := exec.Command(resticCli, "--repo", c.RepoUrl, "restore", snapshot, "--target", path)
	restoreCommand.Env = os.Environ()
	restoreCommand.Env = append(restoreCommand.Env, resticPasswordCommandEnvVar)
	if c.AdditionalEnv != nil {
		restoreCommand.Env = append(restoreCommand.Env, c.AdditionalEnv...)
	}

	if output, err := restoreCommand.CombinedOutput(); err != nil {
		logrus.Error(string(output))
		return true, err
	}

	return true, nil
}

func (c *ResticClient) DownloadLastSnapshot(path string) (bool, error) {
	// Restic automatically finds the newest snapshot when give latest instead of snapshot id
	return c.DownloadSnapshot("latest", path)
}
