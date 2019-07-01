package controller

import (
	"github.com/sam-nguyen7/rbac-permissions-operator/pkg/controller/grouppermission"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, grouppermission.Add)
}
