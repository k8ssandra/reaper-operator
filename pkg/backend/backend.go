package backend

import (
	"errors"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

type CassandraValidationError error

var (
	ClusterNameRequired CassandraValidationError = errors.New("CassandraBackend.ClusterName is required")
)



func Validate(cfg v1alpha1.ServerConfig) error {
	if cfg.StorageType == "" || cfg.StorageType == v1alpha1.Memory {
		return nil
	}

	if cfg.StorageType == v1alpha1.Cassandra {
		if cfg.CassandraBackend == nil || cfg.CassandraBackend.ClusterName == "" {
			return ClusterNameRequired
		}
	}

	return nil
}