package model

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// DatabaseMigrationReplicaCreationIP ...
	DatabaseMigrationReplicaCreationIP = "replica-creation-in-progress"
	// DatabaseMigrationReplicaCreationComplete ..
	DatabaseMigrationReplicaCreationComplete = "replica-creation-complete"

	// DatabaseMigrationReplicaProvisionIP ..
	DatabaseMigrationReplicaProvisionIP = "replica-provision-in-progress"
	// DatabaseMigrationReplicaProvisionComplete ...
	DatabaseMigrationReplicaProvisionComplete = "replica-provision-complete"
)

// DatabaseMigration is the interface for managing Mattermost database migrations.
type DatabaseMigration interface {
	Restore(logger log.FieldLogger) (string, error)
	Status(logger log.FieldLogger) (string, error)
	Teardown(logger log.FieldLogger) error
}

// NotSupportedDatabaseMigration is supplied when systems required a database type that does not
// not support migration. All methods should return an error.
type NotSupportedDatabaseMigration struct{}

// Restore returns not supported database error.
func (n *NotSupportedDatabaseMigration) Restore(logger log.FieldLogger) (string, error) {
	return "", errors.New("attempted to migrate an unsupported database type")
}

// Status returns not supported database error.
func (n *NotSupportedDatabaseMigration) Status(logger log.FieldLogger) (string, error) {
	return "", errors.New("attempted to migrate an unsupported database type")
}

// Teardown returns not supported database error.
func (n *NotSupportedDatabaseMigration) Teardown(logger log.FieldLogger) error {
	return errors.New("attempted to migrate an unsupported database type")
}
