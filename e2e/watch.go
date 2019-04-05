//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package main

import (
	"errors"
	"time"
)

func VerifyCheRunning(status string) (deployed bool, err error) {

	timeout := time.After(15 * time.Minute)
	tick := time.Tick(10 * time.Second)
	for {
		select {
		case <-timeout:
			return false, errors.New("timed out")
		case <-tick:
			customResource, _ := getCR()
			if customResource.Status.CheClusterRunning != status {

			} else {
				return true, nil
			}
		}
	}
}


