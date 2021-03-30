package controller

import (
	"github.com/eclipse-che/che-operator/pkg/controller/checlusterbackup"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, checlusterbackup.Add)
}
