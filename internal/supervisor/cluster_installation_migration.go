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

	case model.CMIStateCreationComplete:
		return s.createClusterInstallationSnapshot(migration, logger)

	case model.CMIStateSnapshotCreationIP:
		return s.waitForSnapshot(migration, logger)

	case model.CMIStateSnapshotCreationComplete:
		return s.restoreDatabase(migration, logger)

	case model.CMIStateRestoreDatabaseIP:
		return s.waitForDatabase(migration, logger)

	case model.CMIStateRestoreDatabaseComplete:
		return model.CMIStateStable

	// case model.CMIStateSnapshotCreationIP:
	// 	return s.createClusterInstallation(migration, logger)
	// case model.CMIStateClusterInstallationCreationIP:
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
		return model.CMIStateCreationFailed
	}

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to get cluster installation: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	if clusterInstallation.LockAcquiredBy == nil {
		clusterInstallationLocked, err := s.clusterInstallationSupervisor.store.LockClusterInstallations([]string{clusterInstallation.ID}, clusterInstallationMigration.ID)
		if err != nil {
			logger.Errorf("unable to lock cluster installation: %s", clusterInstallation.ID)
			return model.CMIStateCreationFailed
		}
		if !clusterInstallationLocked {
			logger.Debugf("still locking cluster installation id: %s", clusterInstallation.ID)
			return model.CMIStateCreationRequested
		}
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to get installation: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	if installation.Database != model.InstallationDatabaseAwsRDS {
		logger.Errorf("migration failed: database %s is not supported", installation.Database)
		return migration.State
	}

	if installation.LockAcquiredBy == nil {
		installationLocked, err := s.installationSupervisor.store.LockInstallation(installation.ID, clusterInstallationMigration.ID)
		if err != nil {
			logger.Errorf("unable to lock installation: %s", err.Error())
			return model.CMIStateCreationFailed
		}
		if !installationLocked {
			logger.Debugf("still locking installation id: %s", installation.ID)
			return migration.State
		}
	}

	return model.CMIStateCreationComplete
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationSnapshot(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve installation: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	err = utils.GetDatabaseMigration(installation, clusterInstallation).Snapshot(logger)
	if err != nil {
		logger.Errorf("failed to create a snapshot of the database: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	return model.CMIStateSnapshotCreationIP
}

// func (s *ClusterInstallationMigrationSupervisor) createClusterInstallation(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {

// 	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}
// 	if clusterInstallationMigration.State != model.CMIStateSnapshotCreationIP {
// 		return model.CMIStateCreationFailed
// 	}

// 	cluster, err := s.clusterSupervisor.store.GetCluster(clusterInstallationMigration.ClusterID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}

// 	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}

// 	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve installation: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}

// 	clusterInstallationRequest := s.installationSupervisor.createClusterInstallation(cluster, installation, s.instanceID, logger)
// 	if clusterInstallationRequest != nil && clusterInstallationRequest.State != model.ClusterStateCreationRequested {
// 		logger.Errorf("unexpected cluster installation state: %s", clusterInstallationRequest.State)
// 		return model.CMIStateCreationFailed
// 	}

// 	err = s.installationSupervisor.Do()
// 	if err != nil {
// 		logger.Errorf("failed when running the scheduler: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}
// 	s.installationSupervisor.Supervise(installation)

// 	return model.CMIStateClusterInstallationCreationIP
// }

// func (s *ClusterInstallationMigrationSupervisor) waitForClusterInstallation(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {

// 	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}
// 	if clusterInstallationMigration.State != model.CMIStateSnapshotCreationIP {
// 		return model.CMIStateCreationFailed
// 	}

// 	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}

// 	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
// 	if err != nil {
// 		logger.Errorf("failed to retrieve installation: %s", err.Error())
// 		return model.CMIStateCreationFailed
// 	}

// 	clusterInstallations, err := s.installationSupervisor.store.GetClusterInstallations(&model.ClusterInstallationFilter{
// 		ClusterID:      migration.ClusterID,
// 		InstallationID: installation.ID,
// 		IncludeDeleted: false,
// 	})
// 	if err != nil || len(clusterInstallations) != 1 {
// 		return model.CMIStateCreationFailed
// 	}

// 	if clusterInstallations[0].State != model.ClusterInstallationStateStable {
// 		logger.Debug("still waiting on cluster installation to become stable")
// 		return migration.State
// 	}

// 	return model.CMIStateClusterInstallationCreationComplete
// }

func (s *ClusterInstallationMigrationSupervisor) waitForSnapshot(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		return model.CMIStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		return model.CMIStateCreationFailed
	}

	snapshotStatus, err := utils.GetDatabaseMigration(installation, clusterInstallation).SnapshotStatus(logger)
	if err != nil {
		logger.Errorf("failed to restore database: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	switch snapshotStatus {
	case model.DatabaseMigrationSnapshotCreationComplete:
		logger.Debug("snapshot creation is completed")
		return model.CMIStateSnapshotCreationComplete
	case model.DatabaseMigrationSnapshotModifying:
		logger.Errorf("snapshot is being modified")
		return model.CMIStateCreationFailed
	case model.DatabaseMigrationSnapshotCreationIP:
		logger.Debug("snapshot creation is still in progress")
	}

	return migration.State
}

func (s *ClusterInstallationMigrationSupervisor) restoreDatabase(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		return model.CMIStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		return model.CMIStateCreationFailed
	}

	snapshotStatus, err := utils.GetDatabaseMigration(installation, clusterInstallation).SnapshotStatus(logger)
	if err != nil {
		logger.Errorf("failed to restore database: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	switch snapshotStatus {
	case model.DatabaseMigrationSnapshotCreationComplete:
		logger.Debug("snapshot is ready - restoring database")
		err = utils.GetDatabaseMigration(installation, clusterInstallation).Restore(logger)
		if err != nil {
			logger.Errorf("failed to restore database: %s", err.Error())
			return model.CMIStateCreationFailed
		}
	case model.DatabaseMigrationSnapshotModifying:
		logger.Errorf("failed to restore database: snapshot is being modified")
		return model.CMIStateCreationFailed
	case model.DatabaseMigrationSnapshotCreationIP:
		logger.Debug("snapshot creation is still in progress")
		return migration.State
	}

	return model.CMIStateRestoreDatabaseIP
}

func (s *ClusterInstallationMigrationSupervisor) waitForDatabase(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		return model.CMIStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		return model.CMIStateCreationFailed
	}

	databaseStatus, err := utils.GetDatabaseMigration(installation, clusterInstallation).DatabaseStatus(logger)
	if err != nil {
		logger.Errorf("failed to restore database: %s", err.Error())
		return model.CMIStateCreationFailed
	}

	switch databaseStatus {
	case model.DatabaseMigrationDatabaseCreationComplete:
		logger.Debug("database creation complete")
		return model.CMIStateRestoreDatabaseComplete
	case model.DatabaseMigrationDatabaseDeletionIP:
		logger.Errorf("failed to restore database: database is being deleted")
		return model.CMIStateCreationFailed
	case model.DatabaseMigrationDatabaseCreationIP:
		logger.Debug("database creation is still in progress")
	}

	return migration.State
}
