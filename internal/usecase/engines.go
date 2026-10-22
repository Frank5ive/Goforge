package usecase

import (
	"fmt"
	"strings"

	"github.com/Frank5ive/goforge/internal/domain"
)

// fileTemplate maps a template to the file it renders.
type fileTemplate struct {
	template string
	path     string
}

// sqlDialect describes how a SQL database spells identifiers, parameters, and
// column types.
type sqlDialect struct {
	quote       func(name string) string
	placeholder func(n int) string // n is 1-based
	types       map[domain.FieldType]string
	idType      string // type of the "id" column
	timeType    string // type of "created_at" and "updated_at"
}

// engine describes everything that differs between the databases GoForge
// supports, so `add db` and `add resource` work the same way for all of them.
type engine struct {
	name  string // on the command line and in .goforge.json, e.g. "postgres"
	label string // for people, e.g. "PostgreSQL"

	module, version string // the Go driver added to go.mod

	dbFiles         []fileTemplate // the db package, relative to the project
	repositoryFiles []fileTemplate // per resource, relative to its package
	dialect         *sqlDialect    // nil for databases without SQL migrations
	repoType        string         // e.g. "PostgresRepository"

	depsImport string // import for the server.Deps field
	depsType   string // type of the server.Deps field, e.g. "*pgxpool.Pool"
	readyCheck string // Go expression returning an error when d.DB is unreachable

	defaultURL func(dbName string) string // DATABASE_URL default for local development
	testURL    func(dbName string) string // TEST_DATABASE_URL for make test-db and CI; nil if tests need none

	composeTemplate string                              // docker-compose.yml template; "" for none
	ciService       func(testDB, testURL string) string // services/env for the CI test job; nil for none
	makeTargets     func(dbName string, testURL string) []string
	gitignore       []string // lines to add to .gitignore
}

// connectCode is inserted into main.go for every engine.
const connectCode = `database, err := db.Connect(ctx, cfg.DatabaseURL, log)
if err != nil {
	return err
}
defer db.Close(database, log)
deps.DB = database
`

var postgresEngine = &engine{
	name:    domain.DatabasePostgres,
	label:   "PostgreSQL",
	module:  "github.com/jackc/pgx/v5",
	version: "v5.11.0",
	dbFiles: []fileTemplate{
		{"db/postgres/db.tmpl", "db/db.go"},
		{"db/postgres/migrate.tmpl", "db/migrate.go"},
		{"db/postgres/migrate_test.tmpl", "db/migrate_test.go"},
		{"db/migrations_readme.tmpl", "db/migrations/README.md"},
		{"db/postgres/dbtest.tmpl", "db/dbtest/dbtest.go"},
	},
	repositoryFiles: []fileTemplate{
		{"db/postgres/repository.tmpl", "postgres.go"},
		{"db/postgres/repository_test.tmpl", "postgres_test.go"},
	},
	dialect: &sqlDialect{
		quote:       func(name string) string { return `"` + name + `"` },
		placeholder: func(n int) string { return fmt.Sprintf("$%d", n) },
		types: map[domain.FieldType]string{
			domain.FieldString: "text",
			domain.FieldInt:    "bigint",
			domain.FieldFloat:  "double precision",
			domain.FieldBool:   "boolean",
			domain.FieldTime:   "timestamptz",
		},
		idType:   "text",
		timeType: "timestamptz",
	},
	repoType:   "PostgresRepository",
	depsImport: "github.com/jackc/pgx/v5/pgxpool",
	depsType:   "*pgxpool.Pool",
	readyCheck: "d.DB.Ping(ctx)",
	defaultURL: func(db string) string {
		return fmt.Sprintf("postgres://app:app@localhost:5432/%s?sslmode=disable", db)
	},
	testURL: func(db string) string {
		return fmt.Sprintf("postgres://app:app@localhost:5432/%s_test?sslmode=disable", db)
	},
	composeTemplate: "db/postgres/compose.tmpl",
	ciService: func(testDB, testURL string) string {
		return fmt.Sprintf(`services:
  postgres:
    image: postgres:18-alpine
    env:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
      POSTGRES_DB: %[1]s
    ports:
      - 5432:5432
    options: >-
      --health-cmd "pg_isready -U app -d %[1]s"
      --health-interval 2s
      --health-timeout 3s
      --health-retries 15
env:
  TEST_DATABASE_URL: %[2]s
`, testDB, testURL)
	},
	makeTargets: func(db, testURL string) []string {
		return dockerMakeTargets("PostgreSQL",
			fmt.Sprintf("-docker compose exec -T db createdb -U app %s_test 2>/dev/null", db), testURL)
	},
}

// engines lists the supported databases by name.
var engines = map[string]*engine{
	postgresEngine.name: postgresEngine,
}

// engineFor returns the engine for a database name from .goforge.json or the
// command line.
func engineFor(name string) (*engine, error) {
	if e, ok := engines[name]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("unsupported database %q", name)
}

// dockerMakeTargets returns Makefile targets for a database run with docker
// compose. prepareTestDB, if not empty, is a recipe line that creates the test
// database.
func dockerMakeTargets(label, prepareTestDB, testURL string) []string {
	lines := []string{
		"db-up: ## Start " + label + " in Docker",
		"\tdocker compose up -d --wait db",
		"",
		"db-down: ## Stop " + label + " (data is kept)",
		"\tdocker compose down",
		"",
		"db-reset: ## Delete all data and start a fresh " + label,
		"\tdocker compose down -v",
		"\tdocker compose up -d --wait db",
		"",
		"test-db: db-up ## Run all tests, including the " + label + " ones",
	}
	if prepareTestDB != "" {
		lines = append(lines, "\t"+prepareTestDB)
	}
	return append(lines, fmt.Sprintf("\tTEST_DATABASE_URL=%q go test ./...", testURL))
}

// phonyTargets returns the target names declared in Makefile target lines.
func phonyTargets(lines []string) []string {
	var names []string
	for _, line := range lines {
		if name, _, ok := strings.Cut(line, ":"); ok && !strings.HasPrefix(line, "\t") && name != "" {
			names = append(names, name)
		}
	}
	return names
}

// addEngineData fills in the database-specific names and statements for a
// resource stored in table. SQL parameters are the ID first, then the fields in
// order, then the timestamps.
func addEngineData(data *resourceData, eng *engine, table string) {
	data.RepoType = eng.repoType
	data.EngineLabel = eng.label
	data.TableName = table

	d := eng.dialect
	if d == nil {
		return
	}

	data.Table = d.quote(table)
	data.IndexName = d.quote(table + "_created_at_id_idx")
	data.IDColumn, data.CreatedColumn, data.UpdatedColumn = d.quote("id"), d.quote("created_at"), d.quote("updated_at")
	data.IDType, data.TimeType = d.idType, d.timeType

	columns := []string{data.IDColumn}
	var sets []string
	for i := range data.Fields {
		f := &data.Fields[i]
		f.Column = d.quote(f.JSON)
		f.SQLType = d.types[f.Type]
		columns = append(columns, f.Column)
		sets = append(sets, fmt.Sprintf("%s = %s", f.Column, d.placeholder(i+2)))
	}
	columns = append(columns, data.CreatedColumn, data.UpdatedColumn)
	data.First = data.Fields[0]

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = d.placeholder(i + 1)
	}

	data.Columns = strings.Join(columns, ", ")
	data.InsertQuery = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		data.Table, data.Columns, strings.Join(placeholders, ", "))
	data.UpdateQuery = fmt.Sprintf("UPDATE %s SET %s, %s = %s WHERE %s = %s",
		data.Table, strings.Join(sets, ", "), data.UpdatedColumn, d.placeholder(len(data.Fields)+2),
		data.IDColumn, d.placeholder(1))
}
