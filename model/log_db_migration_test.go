package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitLogDBMigratesSessionTraceOnSharedSQLite(t *testing.T) {
	backupDB := DB
	backupLOGDB := LOG_DB
	backupUsingSQLite := common.UsingSQLite
	backupIsMasterNode := common.IsMasterNode
	t.Cleanup(func() {
		DB = backupDB
		LOG_DB = backupLOGDB
		common.UsingSQLite = backupUsingSQLite
		common.IsMasterNode = backupIsMasterNode
	})

	tempDir := t.TempDir()
	dbPath := tempDir + "/shared.db"

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.IsMasterNode = true
	t.Setenv("LOG_SQL_DSN", "")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = nil

	require.NoError(t, InitLogDB())
	require.NotNil(t, LOG_DB)
	require.True(t, LOG_DB.Migrator().HasTable(&SessionTrace{}))
	require.True(t, LOG_DB.Migrator().HasTable(&SessionTraceTurn{}))

	_ = os.Remove(dbPath)
}
