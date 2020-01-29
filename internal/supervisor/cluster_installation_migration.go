package supervisor

import (
	"github.com/mattermost/mattermost-cloud/internal/tools/aws"
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
	clusterID                     string
	instanceID                    string
	store                         clusterInstallationMigrationStore
	installationSupervisor        *InstallationSupervisor
	clusterInstallationSupervisor *ClusterInstallationSupervisor
	aws                           aws.AWS
	logger                        log.FieldLogger
}

// NewClusterInstallationMigrationSupervisor creates a new ClusterInstallationMigrationSupervisor.
func NewClusterInstallationMigrationSupervisor(store clusterInstallationMigrationStore, installationSupervisor *InstallationSupervisor,
	clusterInstallationSupervisor *ClusterInstallationSupervisor, aws aws.AWS, instanceID string, logger log.FieldLogger) *ClusterInstallationMigrationSupervisor {
	return &ClusterInstallationMigrationSupervisor{
		store:                         store,
		instanceID:                    instanceID,
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
		s.logger.WithError(err).Warn("Failed to query for migration pending work")
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

	lock := newClusterInstallationMigrationLock(migration.ID, migration.ClusterID, s.store, logger)
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
	case model.InstallationStateCreationRequested, model.InstallationStateCreationNoCompatibleClusters:
		return s.createClusterInstallationMigration(migration, logger)

	default:
		logger.Warnf("Found installation pending work in unexpected state %s", migration.State)
		return migration.State
	}
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationMigration(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	installation, err := s.installationSupervisor.store.GetInstallation(migration.InstallationID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	if installation.State != model.InstallationStateStable {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	err = s.store.CreateClusterInstallationMigration(migration)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	return model.ClusterInstallationMigrationStateCreationRequested
}
