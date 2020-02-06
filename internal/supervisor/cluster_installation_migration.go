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
	case model.ClusterInstallationMigrationStateCreateSnapshot:
		return s.createClusterInstallationSnapshot(migration, logger)
	case model.ClusterInstallationMigrationStateSnapshotCreationIP:
		return s.createClusterInstallation(migration, logger)
	case model.ClusterInstallationMigrationStateClusterInstallationCreated:
		return s.restoreDatabase(migration, logger)
	case model.ClusterInstallationMigrationStateClusterInstallationCreationIP:
		return s.waitForClusterInstallation(migration, logger)

	default:
		logger.Warnf("Found installation pending work in unexpected state %s", migration.State)
		return migration.State
	}
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationMigration(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		logger.Errorf("failed to get cluster installation migration: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to get cluster installation: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	if clusterInstallation.LockAcquiredBy == nil {
		clusterInstallationLocked, err := s.clusterInstallationSupervisor.store.LockClusterInstallations([]string{clusterInstallation.ID}, clusterInstallationMigration.ID)
		if err != nil {
			logger.Errorf("unable to lock cluster installation: %s", clusterInstallation.ID)
			return model.ClusterInstallationMigrationStateCreationFailed
		}
		if !clusterInstallationLocked {
			logger.Debugf("still locking cluster installation id: %s", clusterInstallation.ID)
			return model.ClusterInstallationMigrationStateCreationRequested
		}
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to get installation: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	if installation.Database != model.InstallationDatabaseAwsRDS {
		logger.Errorf("migration failed: database %s is not supported", installation.Database)
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	if installation.LockAcquiredBy == nil {
		installationLocked, err := s.installationSupervisor.store.LockInstallation(installation.ID, clusterInstallationMigration.ID)
		if err != nil {
			logger.Errorf("unable to lock installation: %s", err.Error())
			return model.ClusterInstallationMigrationStateCreationFailed
		}
		if !installationLocked {
			logger.Debugf("still locking installation id: %s", installation.ID)
			return model.ClusterInstallationMigrationStateCreationRequested
		}
	}

	return model.ClusterInstallationMigrationStateCreateSnapshot
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationSnapshot(migration *model.ClusterInstallationMigration,
	logger log.FieldLogger) string {

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve installation: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	err = utils.GetDatabaseMigration(installation, clusterInstallation, logger).Snapshot()
	if err != nil {
		logger.Errorf("failed to create a snapshot of the database: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	return model.ClusterInstallationMigrationStateSnapshotCreationIP
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallation(migration *model.ClusterInstallationMigration,
	logger log.FieldLogger) string {

	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallationMigration.State != model.ClusterInstallationMigrationStateSnapshotCreationIP {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	cluster, err := s.clusterSupervisor.store.GetCluster(clusterInstallationMigration.ClusterID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve installation: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallationRequest := s.installationSupervisor.createClusterInstallation(cluster, installation, s.instanceID, logger)
	if clusterInstallationRequest != nil && clusterInstallationRequest.State != model.ClusterStateCreationRequested {
		logger.Errorf("unexpected cluster installation state: %s", clusterInstallationRequest.State)
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	err = s.installationSupervisor.Do()
	if err != nil {
		logger.Errorf("failed when running the scheduler: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	s.installationSupervisor.Supervise(installation)

	return model.ClusterInstallationMigrationStateClusterInstallationCreationIP
}

func (s *ClusterInstallationMigrationSupervisor) waitForClusterInstallation(migration *model.ClusterInstallationMigration,
	logger log.FieldLogger) string {

	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallationMigration.State != model.ClusterInstallationMigrationStateSnapshotCreationIP {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve cluster installation migration: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		logger.Errorf("failed to retrieve installation: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallations, err := s.installationSupervisor.store.GetClusterInstallations(&model.ClusterInstallationFilter{
		ClusterID:      migration.ClusterID,
		InstallationID: installation.ID,
		IncludeDeleted: false,
	})
	if err != nil || len(clusterInstallations) != 1 {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	if clusterInstallations[0].State != model.ClusterInstallationStateStable {
		logger.Debug("still waiting on cluster installation to become stable")
		return migration.State
	}

	return model.ClusterInstallationMigrationStateClusterInstallationCreated
}

func (s *ClusterInstallationMigrationSupervisor) restoreDatabase(migration *model.ClusterInstallationMigration,
	logger log.FieldLogger) string {

	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallationMigration.State != model.ClusterInstallationMigrationStateClusterInstallationCreationIP {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallation, err := s.clusterInstallationSupervisor.store.GetClusterInstallation(migration.ClusterInstallationID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	installation, err := s.installationSupervisor.store.GetInstallation(clusterInstallation.InstallationID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	clusterInstallations, err := s.installationSupervisor.store.GetClusterInstallations(&model.ClusterInstallationFilter{
		ClusterID:      migration.ClusterID,
		InstallationID: installation.ID,
		IncludeDeleted: false,
	})
	if err != nil || len(clusterInstallations) != 1 {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallations[0].State != model.ClusterInstallationStateStable {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	snapshotStatus, err := utils.GetDatabaseMigration(installation, clusterInstallation, logger, s.aws).SnaphotStatus()
	if err != nil {
		logger.Errorf("failed to restore database: %s", err.Error())
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	switch snapshotStatus {
	case model.DatabaseMigrationSnapshotStatusReady:
		logger.Debug("snapshot is ready - restoring database")
		err = utils.GetDatabaseMigration(installation, clusterInstallation, logger, s.aws).Restore()
		if err != nil {
			logger.Errorf("failed to restore database: %s", err.Error())
			return model.ClusterInstallationMigrationStateCreationFailed
		}

	// TODO(gsagula): perhaps we should just return an error from SnapshotStatus.
	case model.DatabaseMigrationSnapshotStatusFailing:
		logger.Errorf("failed to restore database: snapshot is failing")
		return model.ClusterInstallationMigrationStateCreationFailed

	case model.DatabaseMigrationSnapshotStatusIP:
		logger.Debug("snapshot is still not ready")
		return migration.State
	}

	return model.ClusterInstallationMigrationStateRestoreDatabaseIP
}
