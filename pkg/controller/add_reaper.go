package controller

import (
	"github.com/jsanda/reaper-operator/pkg/controller/reaper"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, reaper.Add)
}
