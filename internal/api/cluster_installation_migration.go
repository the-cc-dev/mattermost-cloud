package api

import (
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-cloud/model"
)

// initMigration registers migration endpoints on the given router.
func initClusterInstallationMigration(apiRouter *mux.Router, context *Context) {
	addContext := func(handler contextHandlerFunc) *contextHandler {
		return newContextHandler(context, handler)
	}

	migrationsRouter := apiRouter.PathPrefix("/migrations").Subrouter()
	migrationsRouter.Handle("", addContext(handleCreateClusterInstallationMigration)).Methods("POST")
}

// handleCreateMigration responds to POST /api/migrations, beginning the process of creating
// a new migration.
func handleCreateClusterInstallationMigration(c *Context, w http.ResponseWriter, r *http.Request) {
	createMigrationRequest, err := model.NewCreateClusterInstallationMigrationRequestFromReader(r.Body)
	if err != nil {
		c.Logger.WithError(err).Error("failed to decode request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	migration := model.ClusterInstallationMigration{
		ClusterID:             createMigrationRequest.ClusterID,
		ClusterInstallationID: createMigrationRequest.ClusterInstallationID,
		State:                 model.ClusterInstallationMigrationStateCreationRequested,
	}

	err = c.Store.CreateClusterInstallationMigration(&migration)
	if err != nil {
		c.Logger.WithError(err).Error("failed to create migration")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Supervisor.Do()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	outputJSON(c, w, migration)
}

// lockMigration synchronizes access to the given cluster across potentially multiple provisioning
// servers.
func lockMigration(c *Context, migrationID string) (*model.ClusterInstallationMigration, int, func()) {
	migration, err := c.Store.GetClusterInstallationMigration(migrationID)
	if err != nil {
		c.Logger.WithError(err).Error("failed to query cluster")
		return nil, http.StatusInternalServerError, nil
	}
	if migration == nil {
		return nil, http.StatusNotFound, nil
	}

	locked, err := c.Store.LockClusterInstallationMigration(migrationID, c.RequestID)
	if err != nil {
		c.Logger.WithError(err).Error("failed to lock cluster")
		return nil, http.StatusInternalServerError, nil
	} else if !locked {
		c.Logger.Error("failed to acquire lock for cluster")
		return nil, http.StatusConflict, nil
	}

	unlockOnce := sync.Once{}

	return migration, 0, func() {
		unlockOnce.Do(func() {
			unlocked, err := c.Store.UnlockClusterInstallationMigration(migration.ID, c.RequestID, false)
			if err != nil {
				c.Logger.WithError(err).Errorf("failed to unlock cluster")
			} else if unlocked != true {
				c.Logger.Error("failed to release lock for cluster")
			}
		})
	}
}
