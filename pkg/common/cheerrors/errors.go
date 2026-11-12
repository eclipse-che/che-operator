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

package cheerrors

import (
	"fmt"
	"time"
)

type RetryError struct {
	Err          error
	Message      string
	RequeueAfter time.Duration
}

func (e *RetryError) Error() string {
	if e.Err != nil {
		if e.Message != "" {
			return fmt.Sprintf("%s: %s", e.Message, e.Err)
		}
		return e.Err.Error()
	}
	return e.Message
}

func (e *RetryError) Unwrap() error {
	return e.Err
}
