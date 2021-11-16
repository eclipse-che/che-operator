//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package util

import (
	"bytes"
	"os/exec"
)

type Runnable interface {
	Run(name string, args ...string) error
	GetStdOut() string
	GetStdErr() string
}

type Process struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func NewRunnable() Runnable {
	return &Process{}
}

func (p *Process) Run(name string, args ...string) error {
	p.cmd = exec.Command(name, args...)
	p.stderr.Reset()
	p.stdout.Reset()
	p.cmd.Stdout = &p.stdout
	p.cmd.Stderr = &p.stderr
	return p.cmd.Run()
}

func (p *Process) GetStdOut() string {
	return string(p.stdout.Bytes())
}

func (p *Process) GetStdErr() string {
	return string(p.stderr.Bytes())
}
