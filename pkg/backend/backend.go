package backend

import (
	"errors"
	"github.com/jsanda/reaper-operator/pkg/apis/reaper/v1alpha1"
)

type CassandraValidationError error

var (
	ClusterNameRequired CassandraValidationError = errors.New("CassandraBackend.ClusterName is required")
	ContactPointsRequired CassandraValidationError = errors.New("CassandraBackend.ContactPoints is required")
)

type BackendService interface {
	Validate(cfg v1alpha1.ServerConfig) error

	CheckDefaults(cfg *v1alpha1.ServerConfig) bool

	InitBackend(r *v1alpha1.Reaper)
}

type defaultBackendService struct{}

func NewBackendService() BackendService {
	return &defaultBackendService{}
}

func (b *defaultBackendService) Validate(cfg v1alpha1.ServerConfig) error {
	if cfg.StorageType == "" || cfg.StorageType == v1alpha1.Memory {
		return nil
	}

	if cfg.StorageType == v1alpha1.Cassandra {
		if cfg.CassandraBackend == nil || cfg.CassandraBackend.ClusterName == "" {
			return ClusterNameRequired
		}

		if len(cfg.CassandraBackend.ContactPoints) == 0 {
			return ContactPointsRequired
		}
	}

	return nil
}

func (b *defaultBackendService) CheckDefaults(cfg *v1alpha1.ServerConfig) bool {
	return false
}

func (b *defaultBackendService) InitBackend(r *v1alpha1.Reaper) {

}