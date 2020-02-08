package supervisor

import (
	awstools "github.com/mattermost/mattermost-cloud/internal/tools/aws"
	"github.com/mattermost/mattermost-cloud/internal/tools/utils"
	"github.com/mattermost/mattermost-cloud/model"
	log "github.com/sirupsen/logrus"
)

// clusterInstallationMigrationStore abstracts the database operations required to query cluster installation migrations.
type clusterInstallationMigrationStore interface {
	GetClusterInstallationMigration(migrationID string) (*model.ClusterInstallationMigration, error)
	GetUnlockedClusterInstallationMigrationsPendingWork() ([]*model.ClusterInstallationMigration, error)
	LockClusterInstallationMigration(migrationID, lockerID string) (bool, error)
	UnlockClusterInstallationMigration(migrationID, lockerID string, force bool) (bool, error)
	DeleteClusterInstallationMigration(migrationID string) error
	CreateClusterInstallationMigration(migration *model.ClusterInstallationMigration) error
	UpdateClusterInstallationMigration(migration *model.ClusterInstallationMigration) error
}

// ClusterInstallationMigrationSupervisor finds migrations pending work and effects the required changes.
//
// The degree of parallelism is controlled by a weighted semaphore, intended to be shared with
// other clients needing to coordinate background jobs.
type ClusterInstallationMigrationSupervisor struct {
	instanceID                    string
	store                         clusterInstallationMigrationStore
	clusterSupervisor             *ClusterSupervisor
	installationSupervisor        *InstallationSupervisor
	clusterInstallationSupervisor *ClusterInstallationSupervisor
	aws                           awstools.AWS
	logger                        log.FieldLogger
}

// NewClusterInstallationMigrationSupervisor creates a new ClusterInstallationMigrationSupervisor.
func NewClusterInstallationMigrationSupervisor(store clusterInstallationMigrationStore, clusterSupervisorInstance *ClusterSupervisor, installationSupervisor *InstallationSupervisor,
	clusterInstallationSupervisor *ClusterInstallationSupervisor, aws awstools.AWS, instanceID string, logger log.FieldLogger) *ClusterInstallationMigrationSupervisor {
	return &ClusterInstallationMigrationSupervisor{
		instanceID:                    instanceID,
		store:                         store,
		clusterSupervisor:             clusterSupervisorInstance,
		installationSupervisor:        installationSupervisor,
		clusterInstallationSupervisor: clusterInstallationSupervisor,
		aws:                           aws,
		logger:                        logger,
	}
}

// Do looks for work to be done on any pending installations and attempts to schedule the required work.
func (s *ClusterInstallationMigrationSupervisor) Do() error {
	migrations, err := s.store.GetUnlockedClusterInstallationMigrationsPendingWork()
	if err != nil {
		s.logger.WithError(err).Warn("Failed to query for cluster installation migration pending work")
		return nil
	}

	for _, migration := range migrations {
		s.Supervise(migration)
	}

	return nil
}

// Supervise schedules the required work on the given migration.
func (s *ClusterInstallationMigrationSupervisor) Supervise(migration *model.ClusterInstallationMigration) {
	logger := s.logger.WithFields(log.Fields{
		"cluster-installation-migration": migration.ID,
	})

	lock := newClusterInstallationMigrationLock(migration.ID, s.instanceID, s.store, logger)
	if !lock.TryLock() {
		return
	}
	defer lock.Unlock()

	logger.Debugf("Supervising cluster installation migration in state %s", migration.State)

	newState := s.transitionClusterInstallationMigration(migration, logger)
	migration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		logger.WithError(err).Warnf("failed to get migration and thus persist state %s", newState)
		return
	}

	if migration.State == newState {
		return
	}

	oldState := migration.State
	migration.State = newState
	err = s.store.UpdateClusterInstallationMigration(migration)
	if err != nil {
		logger.WithError(err).Warnf("Failed to set migration state to %s", newState)
		return
	}

	logger.Debugf("Transitioned installation from %s to %s", oldState, newState)
}

// transitionMigration works with the given migration to migration it to a final state.
func (s *ClusterInstallationMigrationSupervisor) transitionClusterInstallationMigration(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	switch migration.State {
	case model.InstallationStateCreationRequested:
		return s.createClusterInstallationMigration(migration, logger)

	case model.CIMigrationCreationComplete:
		return s.createClusterInstallationSnapshot(migration, logger)

	case model.CIMigrationSnapshotCreationIP:
		return s.restoreDatabase(migration, logger)

	case model.CIMigrationRestoreDatabaseIP:
		return s.waitForDatabase(migration, logger)

	case model.CIMigrationRestoreDatabaseComplete:
		return model.CIMigrationStable

	// case model.CIMigrationSnapshotCreationIP:
	// 	return s.createClusterInstallation(migration, logger)
	// case model.CIMigrationClusterInstallationCreationIP:
	// 	return s.waitForClusterInstallation(migration, logger)

	default:
		logger.Warnf("Found installation pending work in unexpected state %s", migration.State)
		return migration.State
	}
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationMigration(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		logger.Errorf("failed to get cluster installation migration: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to get cluster installation: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	if clusterInstallation.LockAcquiredBy == nil {
		clusterInstallationLocked, err := s.clusterInstallationSupervisor.store.LockClusterInstallations([]string{clusterInstallation.ID}, clusterInstallationMigration.ID)
		if err != nil {
			logger.Errorf("unable to lock cluster installation: %s", clusterInstallation.ID)
			return model.CIMigrationCreationFailed
		}
		if !clusterInstallationLocked {
			logger.Debugf("still locking cluster installation id: %s", clusterInstallation.ID)
			return model.CIMigrationCreationRequested
		}
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to get installation: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	if installation.Database != model.InstallationDatabaseAwsRDS {
		logger.Errorf("migration failed: database %s is not supported", installation.Database)
		return migration.State
	}

	if installation.LockAcquiredBy == nil {
		installationLocked, err := s.installationSupervisor.store.LockInstallation(installation.ID, clusterInstallationMigration.ID)
		if err != nil {
			logger.Errorf("unable to lock installation: %s", err.Error())
			return model.CIMigrationCreationFailed
		}
		if !installationLocked {
			logger.Debugf("still locking installation id: %s", installation.ID)
			return migration.State
		}
	}

	return model.CIMigrationCreationComplete
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationSnapshot(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve installation: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	err = utils.GetDatabase(installation).Snapshot(logger)
	if err != nil {
		logger.Errorf("failed to create a snapshot of the database: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	return model.CIMigrationSnapshotCreationIP
}

// func (s *ClusterInstallationMigrationSupervisor) createClusterInstallation(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {

// 	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}
// 	if clusterInstallationMigration.State != model.CIMigrationSnapshotCreationIP {
// 		return model.CIMigrationCreationFailed
// 	}

// 	cluster, err := s.clusterSupervisor.store.GetCluster(clusterInstallationMigration.ClusterID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}

// 	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}

// 	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve installation: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}

// 	clusterInstallationRequest := s.installationSupervisor.createClusterInstallation(cluster, installation, s.instanceID, logger)
// 	if clusterInstallationRequest != nil && clusterInstallationRequest.State != model.ClusterStateCreationRequested {
// 		logger.Errorf("unexpected cluster installation state: %s", clusterInstallationRequest.State)
// 		return model.CIMigrationCreationFailed
// 	}

// 	err = s.installationSupervisor.Do()
// 	if err != nil {
// 		logger.Errorf("failed when running the scheduler: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}
// 	s.installationSupervisor.Supervise(installation)

// 	return model.CIMigrationClusterInstallationCreationIP
// }

// func (s *ClusterInstallationMigrationSupervisor) waitForClusterInstallation(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {

// 	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}
// 	if clusterInstallationMigration.State != model.CIMigrationSnapshotCreationIP {
// 		return model.CIMigrationCreationFailed
// 	}

// 	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}

// 	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve installation: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}

// 	clusterInstallations, err := s.installationSupervisor.store.GetClusterInstallations(&model.ClusterInstallationFilter{
// 		ClusterID:      migration.ClusterID,
// 		InstallationID: installation.ID,
// 		IncludeDeleted: false,
// 	})
// 	if err != nil || len(clusterInstallations) != 1 {
// 		return model.CIMigrationCreationFailed
// 	}

// 	if clusterInstallations[0].State != model.ClusterInstallationStateStable {
// 		logger.Debug("still waiting on cluster installation to become stable")
// 		return migration.State
// 	}

// 	return model.CIMigrationClusterInstallationCreationComplete
// }

// func (s *ClusterInstallationMigrationSupervisor) waitForSnapshot(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
// 	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
// 	if err != nil {
// 		return model.CIMigrationCreationFailed
// 	}

// 	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
// 	if err != nil {
// 		return model.CIMigrationCreationFailed
// 	}

// 	snapshotStatus, err := utils.GetDatabaseMigration(installation, clusterInstallation).SnapshotStatus(logger)
// 	if err != nil {
// 		logger.Errorf("failed to restore database: %s", err.Error())
// 		return model.CIMigrationCreationFailed
// 	}

// 	switch snapshotStatus {
// 	case model.DatabaseMigrationSnapshotCreationComplete:
// 		logger.Debug("snapshot creation is completed")
// 		return model.CIMigrationSnapshotCreationComplete
// 	case model.DatabaseMigrationSnapshotModifying:
// 		logger.Errorf("snapshot is being modified")
// 		return model.CIMigrationCreationFailed
// 	case model.DatabaseMigrationSnapshotCreationIP:
// 		logger.Debug("snapshot creation is still in progress")
// 	}

// 	return migration.State
// }

func (s *ClusterInstallationMigrationSupervisor) restoreDatabase(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		return model.CIMigrationCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		return model.CIMigrationCreationFailed
	}

	status, err := utils.GetDatabaseMigration(installation, clusterInstallation).Restore(logger)
	if err != nil {
		logger.Errorf("failed to restore database: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	switch status {
	case model.DatabaseMigrationReplicaCreationIP:
		return migration.State
	case model.DatabaseMigrationReplicaCreationComplete:
		return model.CIMigrationRestoreDatabaseIP
	}

	return model.CIMigrationCreationFailed
}

func (s *ClusterInstallationMigrationSupervisor) waitForDatabase(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		return model.CIMigrationCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		return model.CIMigrationCreationFailed
	}

	databaseStatus, err := utils.GetDatabaseMigration(installation, clusterInstallation).Status(logger)
	if err != nil {
		logger.Errorf("failed to restore database: %s", err.Error())
		return model.CIMigrationCreationFailed
	}

	switch databaseStatus {
	case model.DatabaseMigrationReplicaProvisionComplete:
		logger.Debug("database creation complete")
	case model.DatabaseMigrationReplicaProvisionIP:
		logger.Debug("database creation is still in progress")
		return migration.State
	}

	return model.CIMigrationRestoreDatabaseComplete

}
