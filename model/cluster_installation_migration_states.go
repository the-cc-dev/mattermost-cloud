package model

// CMI = Cluster Installation Migration

const (
	// CIMigrationStable is an InstallationMigration in a stable state and undergoing no changes.
	CIMigrationStable = "stable"
	// CIMigrationCreationRequested ...
	CIMigrationCreationRequested = "creation-requested"
	// CIMigrationCreationComplete ...
	CIMigrationCreationComplete = "creation-complete"
	// CIMigrationCreationFailed ...
	CIMigrationCreationFailed = "creation-failed"

	// CIMigrationSnapshotCreationIP indicates that the snapshot is being created.
	CIMigrationSnapshotCreationIP = "snapshot-creation-in-progress"
	// CIMigrationSnapshotCreationComplete waits while the snapshot is being created.
	CIMigrationSnapshotCreationComplete = "snapshot-creation-complete"

	// CIMigrationRestoreDatabaseIP indicates that a database is being restored.
	CIMigrationRestoreDatabaseIP = "restore-database-in-progress"
	// CIMigrationRestoreDatabaseComplete indicates that a database is being restored.
	CIMigrationRestoreDatabaseComplete = "restore-database-complete"

	// CIMigrationClusterInstallationCreationIP indicates that a new cluster installation is being created.
	CIMigrationClusterInstallationCreationIP = "cluster-installation-in-progress"

	// CIMigrationClusterInstallationCreationComplete requests a new cluster installation.
	CIMigrationClusterInstallationCreationComplete = "cluster-installation-creation-complete"

	// CIMigrationCreationInProgress is an InstallationMigration in the process of being created.
	// CIMigrationCreationInProgress = "creation-in-progress"
	// CIMigrationClusterInstallationCreated indicates that a new cluster installation was created.
	// CIMigrationClusterInstallationCreated = "cluster-installation-created"
)

// AllCIMigrations is a list of all states an InstallationMigration can be in.
// Warning:
// When creating a new InstallationMigration state, it must be added to this list.
var AllCIMigrations = []string{
	CIMigrationStable,
	CIMigrationCreationRequested,
	CIMigrationCreationComplete,
	CIMigrationCreationFailed,
	CIMigrationSnapshotCreationIP,
	CIMigrationSnapshotCreationComplete,
	CIMigrationRestoreDatabaseIP,
	CIMigrationRestoreDatabaseComplete,
}

// AllCIMigrationsPendingWork is a list of all InstallationMigration states that
// the supervisor will attempt to transition towards stable on the next "tick".
// Warning:
// When creating a new InstallationMigration state, it must be added to this list if the
// cloud InstallationMigration supervisor should perform some action on its next work cycle.
var AllCIMigrationsPendingWork = []string{
	CIMigrationCreationRequested,
	CIMigrationSnapshotCreationIP,
	CIMigrationCreationRequested,
	CIMigrationCreationComplete,
	CIMigrationSnapshotCreationIP,
	CIMigrationSnapshotCreationComplete,
	CIMigrationRestoreDatabaseIP,
	CIMigrationRestoreDatabaseComplete,
}

// AllCMIRequestStates is a list of all states that an InstallationMigration can
// be put in via the API.
// Warning:
// When creating a new InstallationMigration state, it must be added to this list if an
// API endpoint should put the InstallationMigration in this state.
var AllCMIRequestStates = []string{
	CIMigrationCreationRequested,
}
