package config

import (
	"errors"

	api "github.com/k8ssandra/reaper-operator/api/v1alpha1"
)

type ValidationError error

var (
	DatacenterRequired ValidationError = errors.New("CassandraBackend.CassandraDatacenterRef is required")
)

type Validator interface {
	Validate(reaper *api.Reaper) error

	SetDefaults(reaper *api.Reaper) bool
}

type validator struct{}

func NewValidator() Validator {
	return &validator{}
}

func (v *validator) Validate(reaper *api.Reaper) error {
	cfg := reaper.Spec.ServerConfig

	if cfg.StorageType == "" || cfg.StorageType == api.StorageTypeMemory {
		return nil
	}

	if cfg.StorageType == api.StorageTypeCassandra {
		if cfg.CassandraBackend == nil || len(cfg.CassandraBackend.CassandraDatacenter.Name) == 0 {
			return DatacenterRequired
		}
	}

	return nil
}

func (v *validator) SetDefaults(reaper *api.Reaper) bool {
	updated := false
	cfg := &reaper.Spec.ServerConfig

	if reaper.Spec.Image == "" {
		reaper.Spec.Image = api.DefaultReaperImage
		updated = true
	}

	if cfg.StorageType == "" {
		cfg.StorageType = api.DefaultStorageType
		updated = true
	}

	if cfg.StorageType == api.StorageTypeCassandra {
		cassandra := cfg.CassandraBackend
		if cassandra.Keyspace == "" {
			cassandra.Keyspace = api.DefaultKeyspace
			updated = true
		}

		if cassandra.AuthProvider == (api.AuthProvider{}) {
			cassandra.AuthProvider = api.AuthProvider{
				Type:     "plainText",
				Username: "cassandra",
				Password: "cassandra",
			}
			updated = true
		}

		if cassandra.Replication == (api.ReplicationConfig{}) {
			cassandra.Replication = api.ReplicationConfig{
				SimpleStrategy: int32Ptr(1),
			}
			updated = true
		}
	}

	return updated
}

func int32Ptr(n int32) *int32 {
	return &n
}
