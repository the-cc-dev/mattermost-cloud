package model

// CMI = Cluster Installation Migration

const (
	// CMIStateStable is an InstallationMigration in a stable state and undergoing no changes.
	CMIStateStable = "stable"
	// CMIStateCreationRequested ...
	CMIStateCreationRequested = "creation-requested"
	// CMIStateCreationComplete ...
	CMIStateCreationComplete = "creation-complete"
	// CMIStateCreationFailed ...
	CMIStateCreationFailed = "creation-failed"

	// CMIStateSnapshotCreationIP indicates that the snapshot is being created.
	CMIStateSnapshotCreationIP = "snapshot-creation-in-progress"
	// CMIStateSnapshotCreationComplete waits while the snapshot is being created.
	CMIStateSnapshotCreationComplete = "snapshot-creation-complete"

	// CMIStateRestoreDatabaseIP indicates that a database is being restored.
	CMIStateRestoreDatabaseIP = "restore-database-in-progress"
	// CMIStateRestoreDatabaseComplete indicates that a database is being restored.
	CMIStateRestoreDatabaseComplete = "restore-database-complete"

	// CMIStateClusterInstallationCreationIP indicates that a new cluster installation is being created.
	CMIStateClusterInstallationCreationIP = "cluster-installation-in-progress"

	// CMIStateClusterInstallationCreationComplete requests a new cluster installation.
	CMIStateClusterInstallationCreationComplete = "cluster-installation-creation-complete"

	// CMIStateCreationInProgress is an InstallationMigration in the process of being created.
	// CMIStateCreationInProgress = "creation-in-progress"
	// CMIStateClusterInstallationCreated indicates that a new cluster installation was created.
	// CMIStateClusterInstallationCreated = "cluster-installation-created"
)

// AllCMIStates is a list of all states an InstallationMigration can be in.
// Warning:
// When creating a new InstallationMigration state, it must be added to this list.
var AllCMIStates = []string{
	CMIStateStable,
	CMIStateCreationRequested,
	CMIStateCreationComplete,
	CMIStateCreationFailed,
	CMIStateSnapshotCreationIP,
	CMIStateSnapshotCreationComplete,
	CMIStateRestoreDatabaseIP,
	CMIStateRestoreDatabaseComplete,
}

// AllCMIStatesPendingWork is a list of all InstallationMigration states that
// the supervisor will attempt to transition towards stable on the next "tick".
// Warning:
// When creating a new InstallationMigration state, it must be added to this list if the
// cloud InstallationMigration supervisor should perform some action on its next work cycle.
var AllCMIStatesPendingWork = []string{
	CMIStateCreationRequested,
	CMIStateSnapshotCreationIP,
	CMIStateCreationRequested,
	CMIStateCreationComplete,
	CMIStateSnapshotCreationIP,
	CMIStateSnapshotCreationComplete,
	CMIStateRestoreDatabaseIP,
	CMIStateRestoreDatabaseComplete,
}

// AllCMIRequestStates is a list of all states that an InstallationMigration can
// be put in via the API.
// Warning:
// When creating a new InstallationMigration state, it must be added to this list if an
// API endpoint should put the InstallationMigration in this state.
var AllCMIRequestStates = []string{
	CMIStateCreationRequested,
}
