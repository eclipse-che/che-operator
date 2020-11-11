//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package controller

import (
	"github.com/eclipse/che-operator/pkg/controller/che"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, che.Add)
}
