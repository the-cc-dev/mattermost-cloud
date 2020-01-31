package model

const (
	// ClusterInstallationMigrationStateStable is an InstallationMigration in a stable state and undergoing no changes.
	ClusterInstallationMigrationStateStable = "stable"
	// ClusterInstallationMigrationStateCreationRequested is an InstallationMigration waiting to be created.
	ClusterInstallationMigrationStateCreationRequested = "creation-requested"
	// ClusterInstallationMigrationStateCreateSnapshot creates a snapshot of the cluster installation's database.
	ClusterInstallationMigrationStateCreateSnapshot = "create-snapshot"
	// ClusterInstallationMigrationStateSnapshotCreationIP indicates that the snapshot is being created.
	ClusterInstallationMigrationStateSnapshotCreationIP = "snapshot-creation-in-progress"
	// ClusterInstallationMigrationStateClusterInstallationCreationIP indicates that a new cluster installation is being created.
	ClusterInstallationMigrationStateClusterInstallationCreationIP = "cluster-installation-in-progress"
	// ClusterInstallationMigrationStateWaitForSnapshot waits while the snapshot is being created.
	ClusterInstallationMigrationStateWaitForSnapshot = "waiting-for-snapshot"
	// ClusterInstallationMigrationStateWaitForCluesterInstallation requests a new cluster installation.
	ClusterInstallationMigrationStateWaitForCluesterInstallation = "wait-for-cluster-installation"
	// ClusterInstallationMigrationStateCreationInProgress is an InstallationMigration in the process of being created.
	ClusterInstallationMigrationStateCreationInProgress = "creation-in-progress"
	// ClusterInstallationMigrationStateClusterInstallationCreated indicates that a new cluster installation was created.
	ClusterInstallationMigrationStateClusterInstallationCreated = "cluster-installation-created"
	// ClusterInstallationMigrationStateRestoreDatabase indicates that a new cluster installation was created.
	ClusterInstallationMigrationStateRestoreDatabase = "restore-database"
	// ClusterInstallationMigrationStateRestoreDatabaseIP indicates that a database is being restored.
	ClusterInstallationMigrationStateRestoreDatabaseIP = "restore-database-in-progress"
	// ClusterInstallationMigrationStateCreationFailed is an InstallationMigration that failed creation.
	ClusterInstallationMigrationStateCreationFailed = "creation-failed"
)

// AllClusterInstallationMigrationStates is a list of all states an InstallationMigration can be in.
// Warning:
// When creating a new InstallationMigration state, it must be added to this list.
var AllClusterInstallationMigrationStates = []string{
	ClusterInstallationMigrationStateStable,
	ClusterInstallationMigrationStateCreationRequested,
	ClusterInstallationMigrationStateCreationInProgress,
	ClusterInstallationMigrationStateCreationFailed,
}

// AllClusterInstallationMigrationStatesPendingWork is a list of all InstallationMigration states that
// the supervisor will attempt to transition towards stable on the next "tick".
// Warning:
// When creating a new InstallationMigration state, it must be added to this list if the
// cloud InstallationMigration supervisor should perform some action on its next work cycle.
var AllClusterInstallationMigrationStatesPendingWork = []string{
	ClusterInstallationMigrationStateCreationRequested,
	ClusterInstallationMigrationStateCreationInProgress,
}

// AllClusterInstallationMigrationRequestStates is a list of all states that an InstallationMigration can
// be put in via the API.
// Warning:
// When creating a new InstallationMigration state, it must be added to this list if an
// API endpoint should put the InstallationMigration in this state.
var AllClusterInstallationMigrationRequestStates = []string{
	ClusterInstallationMigrationStateCreationRequested,
}

// ValidTransitionState returns whether an InstallationMigration can be transitioned into
// the new state or not based on its current state.
func (i *ClusterInstallationMigration) ValidTransitionState(newState string) bool {
	switch newState {
	case ClusterInstallationMigrationStateCreationRequested:
		return validTransitionToClusterInstallationMigrationStateCreationRequested(i.State)
	}

	return false
}

func validTransitionToClusterInstallationMigrationStateCreationRequested(currentState string) bool {
	switch currentState {
	case ClusterInstallationMigrationStateCreationRequested,
		ClusterInstallationMigrationStateCreationFailed:
		return true
	}

	return false
}
