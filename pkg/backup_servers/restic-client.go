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
)

const (
	resticCli = "restic"

	resticPasswordCommandEnvVarName = "RESTIC_PASSWORD_COMMAND"
)

type ResticClient struct {
	repoUrl       string
	repoPassword  string
	additionalEnv []string
}

type SnapshotStat struct {
	id   string
	info string
}

func (c *ResticClient) InitRepository() (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.repoPassword)

	initCommand := exec.Command(resticCli, "--repo", c.repoUrl, "init")
	initCommand.Env = os.Environ()
	initCommand.Env = append(initCommand.Env, resticPasswordCommandEnvVar)
	if c.additionalEnv != nil {
		initCommand.Env = append(initCommand.Env, c.additionalEnv...)
	}

	if err := initCommand.Run(); err != nil {
		return true, err
	}

	return true, nil
}

func (c *ResticClient) CheckRepository() (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.repoPassword)

	checkCommand := exec.Command(resticCli, "--repo", c.repoUrl, "check")
	checkCommand.Env = os.Environ()
	checkCommand.Env = append(checkCommand.Env, resticPasswordCommandEnvVar)
	if c.additionalEnv != nil {
		checkCommand.Env = append(checkCommand.Env, c.additionalEnv...)
	}

	if err := checkCommand.Run(); err != nil {
		return true, err
	}

	return true, nil
}

func (c *ResticClient) SendSnapshot(path string) (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.repoPassword)

	// Check that there is data to send
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true, err
	}

	backupCommand := exec.Command(resticCli, "--repo", c.repoUrl, "backup", path)
	backupCommand.Env = os.Environ()
	backupCommand.Env = append(backupCommand.Env, resticPasswordCommandEnvVar)
	if c.additionalEnv != nil {
		backupCommand.Env = append(backupCommand.Env, c.additionalEnv...)
	}

	out, err := backupCommand.Output()
	if err != nil {
		return true, err
	}

	output := string(out)
	stat := SnapshotStat{}
	snapshotIdRegex := regexp.MustCompile("snapshot ([0-9a-f]+) saved")
	snapshotIdMatch := snapshotIdRegex.FindStringSubmatch(output)
	if len(snapshotIdMatch) > 0 {
		stat.id = snapshotIdMatch[1]
	}
	snapshotSizeRegex := regexp.MustCompile("processed (.*)")
	snapshotSizeMatch := snapshotSizeRegex.FindStringSubmatch(output)
	if len(snapshotSizeMatch) > 0 {
		stat.info = snapshotSizeMatch[1]
	}
	// TODO return stat

	return true, nil
}

func (c *ResticClient) DownloadSnapshot(snapshot string, path string) (bool, error) {
	resticPasswordCommandEnvVar := fmt.Sprintf("%s=echo '%s'", resticPasswordCommandEnvVarName, c.repoPassword)

	// Ensure destination path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return true, fmt.Errorf("failed to create directory for snapshot. Reason: %s", err.Error())
		}
	}

	restoreCommand := exec.Command(resticCli, "--repo", c.repoUrl, "restore", snapshot, "--target", path)
	restoreCommand.Env = os.Environ()
	restoreCommand.Env = append(restoreCommand.Env, resticPasswordCommandEnvVar)
	if c.additionalEnv != nil {
		restoreCommand.Env = append(restoreCommand.Env, c.additionalEnv...)
	}

	if err := restoreCommand.Run(); err != nil {
		return true, err
	}

	return true, nil
}

func (c *ResticClient) DownloadLastSnapshot(path string) (bool, error) {
	// Restic automatically finds the newest snapshot when give latest instead of snapshot id
	return c.DownloadSnapshot("latest", path)
}
