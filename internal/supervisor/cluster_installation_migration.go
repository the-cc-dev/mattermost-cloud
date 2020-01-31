package supervisor

import (
	"github.com/mattermost/mattermost-cloud/internal/tools/aws"
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
	clusterID                     string
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
	case model.ClusterInstallationMigrationStateCreateSnapshot:
		return s.createClusterInstallationSnapshot(migration, logger)
	case model.ClusterInstallationMigrationStateSnapshotCreationIP:
		return s.createClusterInstallation(migration, logger)
	case model.ClusterInstallationMigrationStateClusterInstallationCreationIP:
		return s.waitClusterInstallation(migration, logger)
	case model.ClusterInstallationMigrationStateClusterInstallationCreated:
		return s.restoreDatabase(migration, logger)

	default:
		logger.Warnf("Found installation pending work in unexpected state %s", migration.State)
		return migration.State
	}
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationMigration(migration *model.ClusterInstallationMigration, logger log.FieldLogger) string {
	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallationMigration.State != model.ClusterInstallationMigrationStateCreationRequested {
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

	// Lock cluster installation.
	clusterInstallationLocked, err := s.clusterInstallationSupervisor.store.LockClusterInstallations([]string{clusterInstallation.ID}, clusterInstallationMigration.ID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if !clusterInstallationLocked {
		return model.ClusterInstallationMigrationStateCreationRequested
	}

	// Lock installation.
	installationLocked, err := s.installationSupervisor.store.LockInstallation(installation.ID, clusterInstallationMigration.ID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if !installationLocked {
		return model.ClusterInstallationMigrationStateCreationRequested
	}

	// Change state to create snapshot.
	migration.State = model.ClusterInstallationMigrationStateCreateSnapshot
	err = s.store.UpdateClusterInstallationMigration(migration)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreateSnapshot
	}

	return migration.State
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallationSnapshot(migration *model.ClusterInstallationMigration,
	logger log.FieldLogger) string {

	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallationMigration.State != model.ClusterInstallationMigrationStateCreationRequested {
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

	// Create snapshot of the the master installation.
	err = utils.GetDatabase(installation).Snapshot(s.installationSupervisor.store, aws.DefaultMigrationSnapshotTagKey, migration.ClusterInstallationID, logger)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	// Change state to snapshot creation in progress.
	migration.State = model.ClusterInstallationMigrationStateSnapshotCreationIP
	err = s.store.UpdateClusterInstallationMigration(migration)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	return migration.State
}

func (s *ClusterInstallationMigrationSupervisor) createClusterInstallation(migration *model.ClusterInstallationMigration,
	logger log.FieldLogger) string {

	clusterInstallationMigration, err := s.store.GetClusterInstallationMigration(migration.ID)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	if clusterInstallationMigration.State != model.ClusterInstallationMigrationStateSnapshotCreationIP {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	cluster, err := s.clusterSupervisor.store.GetCluster(clusterInstallationMigration.ClusterID)
	if err != nil {
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

	clusterInstallationRequest := s.installationSupervisor.createClusterInstallation(cluster, installation, s.instanceID, logger)
	if clusterInstallationRequest.State != model.ClusterStateCreationRequested {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	err = s.installationSupervisor.Do()
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}
	s.installationSupervisor.Supervise(installation)

	// Change state to cluster installation creation in progress.
	migration.State = model.ClusterInstallationMigrationStateClusterInstallationCreationIP
	err = s.store.UpdateClusterInstallationMigration(migration)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	return migration.State
}

func (s *ClusterInstallationMigrationSupervisor) waitClusterInstallation(migration *model.ClusterInstallationMigration,
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
		return migration.State
	}

	// Change state to cluster installation creation in progress.
	migration.State = model.ClusterInstallationMigrationStateClusterInstallationCreated
	err = s.store.UpdateClusterInstallationMigration(migration)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	return migration.State
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

	newInstallationInstance, err := getInstallationInstance()

	utils.GetDatabase(newInstallationInstance).Restore(s.installationSupervisor.store, aws.DefaultMigrationSnapshotTagKey, migration.ClusterInstallationID)

	// Change state to cluster installation creation in progress.
	migration.State = model.ClusterInstallationMigrationStateClusterInstallationCreated
	err = s.store.UpdateClusterInstallationMigration(migration)
	if err != nil {
		return model.ClusterInstallationMigrationStateCreationFailed
	}

	return migration.State
}
