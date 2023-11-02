//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

// Package solver contains the implementation of the "devworkspace routing solver" which provides che-specific
// logic to the otherwise generic dev workspace routing controller.
// The devworkspace routing controller needs to be provided with a "solver getter" in its configuration prior
// to starting the reconciliation loop. See `CheRouterGetter`.
package solver
