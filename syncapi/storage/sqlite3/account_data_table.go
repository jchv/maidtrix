// Copyright 2017-2018 New Vector Ltd
// Copyright 2019-2020 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlite3

import (
	"context"
	"database/sql"

	"github.com/jchv/maidtrix/internal"
	"github.com/jchv/maidtrix/internal/sqlutil"
	"github.com/jchv/maidtrix/syncapi/storage/tables"
	"github.com/jchv/maidtrix/syncapi/synctypes"
	"github.com/jchv/maidtrix/syncapi/types"
)

const accountDataSchema = `
CREATE TABLE IF NOT EXISTS syncapi_account_data_type (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    type TEXT NOT NULL,
    UNIQUE (user_id, room_id, type)
);
`

const insertAccountDataSQL = "" +
	"INSERT INTO syncapi_account_data_type (id, user_id, room_id, type) VALUES ($1, $2, $3, $4)" +
	" ON CONFLICT (user_id, room_id, type) DO UPDATE" +
	" SET id = $5"

// further parameters are added by prepareWithFilters
const selectAccountDataInRangeSQL = "" +
	"SELECT id, room_id, type FROM syncapi_account_data_type" +
	" WHERE user_id = $1 AND id > $2 AND id <= $3"

const selectMaxAccountDataIDSQL = "" +
	"SELECT MAX(id) FROM syncapi_account_data_type"

type accountDataStatements struct {
	db                           *sql.DB
	streamIDStatements           *StreamIDStatements
	insertAccountDataStmt        *sql.Stmt
	selectMaxAccountDataIDStmt   *sql.Stmt
	selectAccountDataInRangeStmt *sql.Stmt
}

func NewSqliteAccountDataTable(db *sql.DB, streamID *StreamIDStatements) (tables.AccountData, error) {
	s := &accountDataStatements{
		db:                 db,
		streamIDStatements: streamID,
	}
	_, err := db.Exec(accountDataSchema)
	if err != nil {
		return nil, err
	}
	return s, sqlutil.StatementList{
		{&s.insertAccountDataStmt, insertAccountDataSQL},
		{&s.selectMaxAccountDataIDStmt, selectMaxAccountDataIDSQL},
		{&s.selectAccountDataInRangeStmt, selectAccountDataInRangeSQL},
	}.Prepare(db)
}

func (s *accountDataStatements) InsertAccountData(
	ctx context.Context, txn *sql.Tx,
	userID, roomID, dataType string,
) (pos types.StreamPosition, err error) {
	pos, err = s.streamIDStatements.nextAccountDataID(ctx, txn)
	if err != nil {
		return
	}
	_, err = sqlutil.TxStmt(txn, s.insertAccountDataStmt).ExecContext(ctx, pos, userID, roomID, dataType, pos)
	return
}

func (s *accountDataStatements) SelectAccountDataInRange(
	ctx context.Context, txn *sql.Tx,
	userID string,
	r types.Range,
	filter *synctypes.EventFilter,
) (data map[string][]string, pos types.StreamPosition, err error) {
	data = make(map[string][]string)
	stmt, params, err := prepareWithFilters(
		s.db, txn, selectAccountDataInRangeSQL,
		[]interface{}{
			userID, r.Low(), r.High(),
		},
		filter.Senders, filter.NotSenders,
		filter.Types, filter.NotTypes,
		[]string{}, nil, filter.Limit, FilterOrderAsc)
	if err != nil {
		return
	}
	rows, err := stmt.QueryContext(ctx, params...)
	if err != nil {
		return
	}
	defer internal.CloseAndLogIfError(ctx, rows, "selectAccountDataInRange: rows.close() failed")

	var dataType string
	var roomID string
	var id types.StreamPosition

	for rows.Next() {
		if err = rows.Scan(&id, &roomID, &dataType); err != nil {
			return
		}

		if len(data[roomID]) > 0 {
			data[roomID] = append(data[roomID], dataType)
		} else {
			data[roomID] = []string{dataType}
		}
		if id > pos {
			pos = id
		}
	}
	if pos == 0 {
		pos = r.High()
	}
	return data, pos, rows.Err()
}

func (s *accountDataStatements) SelectMaxAccountDataID(
	ctx context.Context, txn *sql.Tx,
) (id int64, err error) {
	var nullableID sql.NullInt64
	err = sqlutil.TxStmt(txn, s.selectMaxAccountDataIDStmt).QueryRowContext(ctx).Scan(&nullableID)
	if nullableID.Valid {
		id = nullableID.Int64
	}
	return
}
