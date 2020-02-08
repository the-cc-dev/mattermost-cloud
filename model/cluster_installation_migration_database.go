package model

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// DatabaseMigrationSnapshotCreationComplete ...
	DatabaseMigrationSnapshotCreationComplete = "snapshot-creation-complete"
	// DatabaseMigrationSnapshotCreationIP ...
	DatabaseMigrationSnapshotCreationIP = "snapshot-creation-in-progress"
	// DatabaseMigrationSnapshotModifying ...
	DatabaseMigrationSnapshotModifying = "snapshot-modifying"

	// DatabaseMigrationDatabaseCreationComplete ..
	DatabaseMigrationDatabaseCreationComplete = "-databasecreation-complete"
	// DatabaseMigrationDatabaseCreationIP ...
	DatabaseMigrationDatabaseCreationIP = "database-creation-in-progress"
	// DatabaseMigrationDatabaseDeletionIP ...
	DatabaseMigrationDatabaseDeletionIP = "database-deletion-in-progress"

	// NotSupportedDatabaseErrorMessage is use to report that database type does not
	// support migration.
	NotSupportedDatabaseErrorMessage = "attempted to migrate an unsupported database type"
)

// DatabaseMigration is the interface for managing Mattermost database migrations.
type DatabaseMigration interface {
	Restore(logger log.FieldLogger) error
	Snapshot(logger log.FieldLogger) error
	SnapshotStatus(logger log.FieldLogger) (string, error)
	DatabaseStatus(logger log.FieldLogger) (string, error)
}

// NotSupportedDatabaseMigration is supplied when systems required a database type that does not
// not support migration. All methods should return an error.
type NotSupportedDatabaseMigration struct{}

// Restore returns not supported database error.
func (n *NotSupportedDatabaseMigration) Restore(logger log.FieldLogger) error {
	return errors.New(NotSupportedDatabaseErrorMessage)
}

// Snapshot returns not supported database error.
func (n *NotSupportedDatabaseMigration) Snapshot(logger log.FieldLogger) error {
	return errors.New(NotSupportedDatabaseErrorMessage)
}

// SnapshotStatus returns not supported database error.
func (n *NotSupportedDatabaseMigration) SnapshotStatus(logger log.FieldLogger) (string, error) {
	return "", errors.New(NotSupportedDatabaseErrorMessage)
}

// DatabaseStatus returns not supported database error.
func (n *NotSupportedDatabaseMigration) DatabaseStatus(logger log.FieldLogger) (string, error) {
	return "", errors.New(NotSupportedDatabaseErrorMessage)
}
