package controller

import (
	"github.com/jsanda/cassandrareaper-operator/pkg/controller/cassandrareaper"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, cassandrareaper.Add)
}
