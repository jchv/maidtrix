// Copyright 2022 The Matrix.org Foundation C.I.C.
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

package test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type DBType int

var DBTypeSQLite DBType = 1
var DBTypePostgres DBType = 2

var Quiet = false
var Required = os.Getenv("DENDRITE_TEST_SKIP_NODB") == ""

func fatalError(t *testing.T, format string, args ...interface{}) {
	if Required {
		t.Fatalf(format, args...)
	} else {
		t.Skipf(format, args...)
	}
}

func createLocalDB(t *testing.T, dbName string) (connStr string) {
	pgContainer, err := postgres.Run(context.Background(),
		"docker.io/postgres:alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		fatalError(t, "error creating local database: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Fatalf("failed to terminate pgContainer: %s", err)
		}
	})
	connStr, err = pgContainer.ConnectionString(context.Background(), "sslmode=disable")
	if err != nil {
		fatalError(t, "error getting connection string for local database: %v", err)
	}
	return connStr
}

func createRemoteDB(t *testing.T, dbName, user, connStr string) {
	db, err := sql.Open("postgres", connStr+" dbname=postgres")
	if err != nil {
		fatalError(t, "failed to open postgres conn with connstr=%s : %s", connStr, err)
	}
	if err = db.Ping(); err != nil {
		fatalError(t, "failed to open postgres conn with connstr=%s : %s", connStr, err)
	}
	_, err = db.Exec(fmt.Sprintf(`CREATE DATABASE %s;`, dbName))
	if err != nil {
		pqErr, ok := err.(*pq.Error)
		if !ok {
			t.Fatalf("failed to CREATE DATABASE: %s", err)
		}
		// we ignore duplicate database error as we expect this
		if pqErr.Code != "42P04" {
			t.Fatalf("failed to CREATE DATABASE with code=%s msg=%s", pqErr.Code, pqErr.Message)
		}
	}
	_, err = db.Exec(fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE %s TO %s`, dbName, user))
	if err != nil {
		t.Fatalf("failed to GRANT: %s", err)
	}
	_ = db.Close()
}

func currentUser() string {
	user, err := user.Current()
	if err != nil {
		if !Quiet {
			fmt.Println("cannot get current user: ", err)
		}
		os.Exit(2)
	}
	return user.Username
}

// Prepare a sqlite or postgres connection string for testing.
// Returns the connection string to use and a close function which must be called when the test finishes.
// Calling this function twice will return the same database, which will have data from previous tests
// unless close() is called.
func PrepareDBConnectionString(t *testing.T, dbType DBType) (connStr string, close func()) {
	if dbType == DBTypeSQLite {
		// this will be made in the t.TempDir, which is unique per test
		dbname := filepath.Join(t.TempDir(), "dendrite_test.db")
		return fmt.Sprintf("file:%s", dbname), func() {
			t.Cleanup(func() {}) // removes the t.TempDir
		}
	}

	// Required vars: user and db
	// We'll try to infer from the local env if they are missing
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = currentUser()
	}
	connStr = fmt.Sprintf(
		"user=%s sslmode=disable",
		user,
	)
	// optional vars, used in CI
	password := os.Getenv("POSTGRES_PASSWORD")
	if password != "" {
		connStr += fmt.Sprintf(" password=%s", password)
	}
	host := os.Getenv("POSTGRES_HOST")
	if host != "" {
		connStr += fmt.Sprintf(" host=%s", host)
	}

	// superuser database
	postgresDB := os.Getenv("POSTGRES_DB")
	// we cannot use 'dendrite_test' here else 2x concurrently running packages will try to use the same db.
	// instead, hash the current working directory, snaffle the first 16 bytes and append that to dendrite_test
	// and use that as the unique db name. We do this because packages are per-directory hence by hashing the
	// working (test) directory we ensure we get a consistent hash and don't hash against concurrent packages.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get working directory: %s", err)
	}
	hash := sha256.Sum256([]byte(wd))
	dbName := fmt.Sprintf("dendrite_test_%s", hex.EncodeToString(hash[:16]))
	if postgresDB == "" { // use testcontainers
		connStr = createLocalDB(t, dbName)
	} else { // remote server, shell into the postgres user and CREATE DATABASE
		createRemoteDB(t, dbName, user, connStr)
		connStr += fmt.Sprintf(" dbname=%s", dbName)
	}

	return connStr, func() {
		// Drop all tables on the database to get a fresh instance
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			t.Fatalf("failed to connect to postgres db '%s': %s", connStr, err)
		}
		_, err = db.Exec(`DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;`)
		if err != nil {
			t.Fatalf("failed to cleanup postgres db '%s': %s", connStr, err)
		}
		_ = db.Close()
	}
}

// Creates subtests with each known DBType
func WithAllDatabases(t *testing.T, testFn func(t *testing.T, db DBType)) {
	dbs := map[string]DBType{
		"postgres": DBTypePostgres,
		"sqlite":   DBTypeSQLite,
	}
	for dbName, dbType := range dbs {
		dbt := dbType
		t.Run(dbName, func(tt *testing.T) {
			tt.Parallel()
			testFn(tt, dbt)
		})
	}
}
