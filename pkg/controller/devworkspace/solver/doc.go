// Package solver contains the implementation of the "devworkspace routing solver" which provides che-specific
// logic to the otherwise generic dev workspace routing controller.
// The devworkspace routing controller needs to be provided with a "solver getter" in its configuration prior
// to starting the reconciliation loop. See `CheRouterGetter`.
package solver
