package model

const (
	// DatabaseMigrationSnapshotStatusReady ...
	DatabaseMigrationSnapshotStatusReady = "snapshot-ready"
	// DatabaseMigrationSnapshotStatusIP ...
	DatabaseMigrationSnapshotStatusIP = "snapshot-in-progress"
	// DatabaseMigrationSnapshotStatusFailing ...
	DatabaseMigrationSnapshotStatusFailing = "snapshot-failing"

	// DatabaseMigrationRestoreComplete ..
	DatabaseMigrationRestoreComplete = "restore-complete"
	// DatabaseMigrationRestoreFailing ..
	DatabaseMigrationRestoreFailing = "restore-failing"
	// DatabaseMigrationRestoreInProgress ...
	DatabaseMigrationRestoreInProgress = "restore-in-progress"
)

// DatabaseMigration is the interface for managing Mattermost database migrations.
type DatabaseMigration interface {
	Restore() error
	Snapshot() error
	SnaphotStatus() (string, error)
	DatabaseStatus() (string, error)
}
