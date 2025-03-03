package tables_test

import (
	"context"
	"testing"

	"github.com/jchv/maidtrix/internal/sqlutil"
	"github.com/jchv/maidtrix/internal/util"
	"github.com/jchv/maidtrix/roomserver/storage/postgres"
	"github.com/jchv/maidtrix/roomserver/storage/sqlite3"
	"github.com/jchv/maidtrix/roomserver/storage/tables"
	"github.com/jchv/maidtrix/setup/config"
	"github.com/jchv/maidtrix/test"
	"github.com/stretchr/testify/assert"
)

func mustCreatePreviousEventsTable(t *testing.T, dbType test.DBType) (tab tables.PreviousEvents, close func()) {
	t.Helper()
	connStr, close := test.PrepareDBConnectionString(t, dbType)
	db, err := sqlutil.Open(&config.DatabaseOptions{
		ConnectionString: config.DataSource(connStr),
	}, sqlutil.NewExclusiveWriter())
	assert.NoError(t, err)
	switch dbType {
	case test.DBTypePostgres:
		err = postgres.CreatePrevEventsTable(db)
		assert.NoError(t, err)
		tab, err = postgres.PreparePrevEventsTable(db)
	case test.DBTypeSQLite:
		err = sqlite3.CreatePrevEventsTable(db)
		assert.NoError(t, err)
		tab, err = sqlite3.PreparePrevEventsTable(db)
	}
	assert.NoError(t, err)

	return tab, close
}

func TestPreviousEventsTable(t *testing.T) {
	ctx := context.Background()
	alice := test.NewUser(t)
	room := test.NewRoom(t, alice)
	test.WithAllDatabases(t, func(t *testing.T, dbType test.DBType) {
		tab, close := mustCreatePreviousEventsTable(t, dbType)
		defer close()

		for _, x := range room.Events() {
			for _, eventID := range x.PrevEventIDs() {
				err := tab.InsertPreviousEvent(ctx, nil, eventID, 1)
				assert.NoError(t, err)

				err = tab.SelectPreviousEventExists(ctx, nil, eventID)
				assert.NoError(t, err)
			}
		}

		// RandomString should fail and return sql.ErrNoRows
		err := tab.SelectPreviousEventExists(ctx, nil, util.RandomString(16))
		assert.Error(t, err)
	})
}
