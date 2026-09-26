// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/migrate"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/tempdb"
	"storj.io/storj/shared/tagsql"
)

func TestBasicMigrationSqliteNoRebind(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(ctx, t, db, db)
}

func TestBasicMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	basicMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestBasicMigration(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "create-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { assert.NoError(t, db.Close()) }()

		basicMigration(ctx, t, db.DB, &postgresDB{DB: db.DB})
	})
}

func basicMigration(ctx *testcontext.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + strings.ReplaceAll(t.Name(), "/", "_"))
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName, "users")) }()

	/* #nosec G306 */ // This is a test besides the file contains just test data.
	err := os.WriteFile(ctx.File("alpha.txt"), []byte("test"), 0o644)
	require.NoError(t, err)
	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          &testDB,
				Description: "Initialize Table",
				Version:     1,
				Action: migrate.SQL{
					`CREATE TABLE users (id int)`,
					`INSERT INTO users (id) VALUES (1)`,
				},
			},
			{
				DB:          &testDB,
				Description: "Move files",
				Version:     2,
				Action: migrate.Func(func(_ context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					return os.Rename(ctx.File("alpha.txt"), ctx.File("beta.txt"))
				}),
			},
		},
	}

	dbVersion, err := m.CurrentVersion(ctx, nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, -1)

	err = m.Run(ctx, zap.NewNop())
	assert.NoError(t, err)

	dbVersion, err = m.CurrentVersion(ctx, nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, 2)

	m2 := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:      &testDB,
				Version: 3,
			},
		},
	}
	dbVersion, err = m2.CurrentVersion(ctx, nil, testDB)
	assert.NoError(t, err)
	assert.Equal(t, dbVersion, 2)

	var version int
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	err = db.QueryRowContext(ctx, `SELECT MAX(version) FROM `+dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	var id int
	err = db.QueryRowContext(ctx, `SELECT MAX(id) FROM users`).Scan(&id)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)

	// file not exists
	_, err = os.Stat(ctx.File("alpha.txt"))
	assert.Error(t, err)

	// file exists
	_, err = os.Stat(ctx.File("beta.txt"))
	assert.NoError(t, err)
	data, err := os.ReadFile(ctx.File("beta.txt"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("test"), data)
}

func TestMultipleMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	multipleMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestMultipleMigrationPostgres(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	connstr := dbtest.PickPostgres(t)

	db, err := tagsql.Open(ctx, "pgx", connstr, nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	multipleMigration(ctx, t, db, &postgresDB{DB: db})
}

func multipleMigration(ctx context.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName)) }()

	steps := 0
	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          &testDB,
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					steps++
					return nil
				}),
			},
			{
				DB:          &testDB,
				Description: "Step 2",
				Version:     2,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					steps++
					return nil
				}),
			},
		},
	}

	err := m.Run(ctx, zap.NewNop())
	assert.NoError(t, err)
	assert.Equal(t, 2, steps)

	m.Steps = append(m.Steps, &migrate.Step{
		DB:          &testDB,
		Description: "Step 3",
		Version:     3,
		Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
			steps++
			return nil
		}),
	})
	err = m.Run(ctx, zap.NewNop())
	assert.NoError(t, err)

	var version int
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	err = db.QueryRowContext(ctx, `SELECT MAX(version) FROM `+dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 3, version)

	assert.Equal(t, 3, steps)
}

func TestFailedMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	failedMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestFailedMigrationPostgres(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	connstr := dbtest.PickPostgres(t)

	db, err := tagsql.Open(ctx, "pgx", connstr, nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	failedMigration(ctx, t, db, &postgresDB{DB: db})
}

func failedMigration(ctx context.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + t.Name())
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName)) }()

	m := migrate.Migration{
		Table: dbName,
		Steps: []*migrate.Step{
			{
				DB:          &testDB,
				Description: "Step 1",
				Version:     1,
				Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
					return errors.New("migration failed")
				}),
			},
		},
	}

	err := m.Run(ctx, zap.NewNop())
	require.Error(t, err, "migration failed")

	var version sql.NullInt64
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	err = db.QueryRowContext(ctx, `SELECT MAX(version) FROM `+dbName).Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, false, version.Valid)
}

func TestNamespaceMigrationSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	namespaceMigration(ctx, t, db, &sqliteDB{DB: db})
}

func TestNamespaceMigration(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "namespace-")
		require.NoError(t, err)
		defer func() { assert.NoError(t, db.Close()) }()

		namespaceMigration(ctx, t, db.DB, &postgresDB{DB: db.DB})
	})
}

// countingMigration returns a migration with the given number of steps, which increments counter on each executed step.
func countingMigration(table, namespace string, testDB *tagsql.DB, stepCount int, counter *int) migrate.Migration {
	m := migrate.Migration{
		Table:     table,
		Namespace: namespace,
	}
	for version := 1; version <= stepCount; version++ {
		m.Steps = append(m.Steps, &migrate.Step{
			DB:          testDB,
			Description: "Step " + strconv.Itoa(version),
			Version:     version,
			Action: migrate.Func(func(ctx context.Context, log *zap.Logger, _ tagsql.DB, tx tagsql.Tx) error {
				*counter++
				return nil
			}),
		})
	}
	return m
}

func namespaceMigration(ctx context.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + strings.ReplaceAll(t.Name(), "/", "_"))
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName)) }()

	var defaultSteps, fooSteps, barSteps int
	defaultMigration := countingMigration(dbName, "", &testDB, 3, &defaultSteps)
	fooMigration := countingMigration(dbName, "foo", &testDB, 2, &fooSteps)
	barMigration := countingMigration(dbName, "bar", &testDB, 1, &barSteps)

	require.NoError(t, defaultMigration.Run(ctx, zap.NewNop()))
	require.Equal(t, 3, defaultSteps)

	// other namespaces are not initialized yet, even if they use the same table
	version, err := fooMigration.CurrentVersion(ctx, nil, testDB)
	require.NoError(t, err)
	require.Equal(t, -1, version)

	require.NoError(t, fooMigration.Run(ctx, zap.NewNop()))
	require.Equal(t, 2, fooSteps)

	require.NoError(t, barMigration.Run(ctx, zap.NewNop()))
	require.Equal(t, 1, barSteps)

	for _, tc := range []struct {
		migration migrate.Migration
		version   int
	}{
		{defaultMigration, 3},
		{fooMigration, 2},
		{barMigration, 1},
	} {
		version, err := tc.migration.CurrentVersion(ctx, nil, testDB)
		require.NoError(t, err)
		require.Equal(t, tc.version, version, "namespace %q", tc.migration.Namespace)
		require.NoError(t, tc.migration.ValidateVersions(ctx, zap.NewNop()))
	}

	// re-running migrations doesn't execute any steps again
	require.NoError(t, defaultMigration.Run(ctx, zap.NewNop()))
	require.NoError(t, fooMigration.Run(ctx, zap.NewNop()))
	require.NoError(t, barMigration.Run(ctx, zap.NewNop()))
	require.Equal(t, 3, defaultSteps)
	require.Equal(t, 2, fooSteps)
	require.Equal(t, 1, barSteps)
}

func TestLegacyVersionTableSqlite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, db.Close()) }()

	legacyVersionTable(ctx, t, db, &sqliteDB{DB: db})
}

func TestLegacyVersionTable(t *testing.T) {
	dbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, connstr string) {
		db, err := tempdb.OpenUnique(ctx, zaptest.NewLogger(t), connstr, "legacy-")
		require.NoError(t, err)
		defer func() { assert.NoError(t, db.Close()) }()

		legacyVersionTable(ctx, t, db.DB, &postgresDB{DB: db.DB})
	})
}

func legacyVersionTable(ctx context.Context, t *testing.T, db tagsql.DB, testDB tagsql.DB) {
	dbName := strings.ToLower(`versions_` + strings.ReplaceAll(t.Name(), "/", "_"))
	defer func() { assert.NoError(t, dropTables(ctx, db, dbName)) }()

	// version table, as it was created before the namespace column was introduced
	/* #nosec G202 */ // This is a test besides the dbName value is generated in
	// a controlled way
	_, err := db.ExecContext(ctx, `CREATE TABLE `+dbName+` (version int, commited_at text)`) //nolint:misspell
	require.NoError(t, err)
	for _, version := range []string{"1", "2"} {
		_, err = db.ExecContext(ctx, `INSERT INTO `+dbName+` (version, commited_at) VALUES (`+version+`, 'now')`) //nolint:misspell
		require.NoError(t, err)
	}

	var defaultSteps, fooSteps int
	defaultMigration := countingMigration(dbName, "", &testDB, 3, &defaultSteps)
	fooMigration := countingMigration(dbName, "foo", &testDB, 2, &fooSteps)

	// existing versions belong to the default namespace
	version, err := defaultMigration.CurrentVersion(ctx, nil, testDB)
	require.NoError(t, err)
	require.Equal(t, 2, version)

	// adding the column is idempotent
	version, err = defaultMigration.CurrentVersion(ctx, nil, testDB)
	require.NoError(t, err)
	require.Equal(t, 2, version)

	require.NoError(t, defaultMigration.Run(ctx, zap.NewNop()))
	require.Equal(t, 1, defaultSteps, "only the missing step should be executed")

	require.NoError(t, fooMigration.Run(ctx, zap.NewNop()))
	require.Equal(t, 2, fooSteps)

	version, err = defaultMigration.CurrentVersion(ctx, nil, testDB)
	require.NoError(t, err)
	require.Equal(t, 3, version)

	version, err = fooMigration.CurrentVersion(ctx, nil, testDB)
	require.NoError(t, err)
	require.Equal(t, 2, version)
}

func TestTargetVersion(t *testing.T) {
	m := migrate.Migration{
		Table: "test",
		Steps: []*migrate.Step{
			{
				Description: "Step 1",
				Version:     1,
				Action:      migrate.SQL{},
			},
			{
				Description: "Step 2",
				Version:     2,
				Action:      migrate.SQL{},
			},
			{
				Description: "Step 2.2",
				Version:     2,
				Action:      migrate.SQL{},
			},
			{
				Description: "Step 3",
				Version:     3,
				Action:      migrate.SQL{},
			},
		},
	}
	testedMigration := m.TargetVersion(2)
	assert.Equal(t, 3, len(testedMigration.Steps))
}

func TestInvalidStepsOrder(t *testing.T) {
	m := migrate.Migration{
		Table: "test",
		Steps: []*migrate.Step{
			{
				Version: 0,
			},
			{
				Version: 1,
			},
			{
				Version: 4,
			},
			{
				Version: 2,
			},
		},
	}

	err := m.ValidateSteps()
	require.Error(t, err, "migrate: steps have incorrect order")
}

func dropTables(ctx context.Context, db tagsql.DB, names ...string) error {
	var errlist errs.Group
	for _, name := range names {
		_, err := db.ExecContext(ctx, `DROP TABLE `+name)
		errlist.Add(err)
	}

	return errlist.Err()
}
