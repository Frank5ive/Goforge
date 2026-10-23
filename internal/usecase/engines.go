package usecase

import (
	"fmt"
	"strconv"
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
	idType      string                    // type of the "id" column
	timeType    string                    // type of "created_at" and "updated_at"
	clear       func(table string) string // empties a table between tests
	textTimes   bool                      // times are stored as fixed-width UTC text
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
	dockerNote      string   // shown when the project has a Dockerfile
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
	label:   domain.DatabaseLabel(domain.DatabasePostgres),
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
		clear:    func(table string) string { return "TRUNCATE " + table },
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
      --health-cmd "pg_isready -U app -d %[1]s -h 127.0.0.1"
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

// sqlFiles are the db package files shared by the database/sql engines.
func sqlFiles(engine string) []fileTemplate {
	return []fileTemplate{
		{"db/" + engine + "/db.tmpl", "db/db.go"},
		{"db/sql/migrate.tmpl", "db/migrate.go"},
		{"db/sql/migrate_test.tmpl", "db/migrate_test.go"},
		{"db/migrations_readme.tmpl", "db/migrations/README.md"},
		{"db/sql/dbtest.tmpl", "db/dbtest/dbtest.go"},
	}
}

var mysqlEngine = &engine{
	name:    domain.DatabaseMySQL,
	label:   domain.DatabaseLabel(domain.DatabaseMySQL),
	module:  "github.com/go-sql-driver/mysql",
	version: "v1.10.1",
	dbFiles: sqlFiles("mysql"),
	repositoryFiles: []fileTemplate{
		{"db/sql/repository.tmpl", "mysql.go"},
		{"db/sql/repository_test.tmpl", "mysql_test.go"},
	},
	dialect: &sqlDialect{
		quote:       func(name string) string { return "`" + name + "`" },
		placeholder: func(int) string { return "?" },
		types: map[domain.FieldType]string{
			domain.FieldString: "TEXT",
			domain.FieldInt:    "BIGINT",
			domain.FieldFloat:  "DOUBLE",
			domain.FieldBool:   "BOOLEAN",
			domain.FieldTime:   "DATETIME(6)",
		},
		idType:   "VARCHAR(64)",
		timeType: "DATETIME(6)",
		clear:    func(table string) string { return "TRUNCATE TABLE " + table },
	},
	repoType:        "MySQLRepository",
	depsImport:      "database/sql",
	depsType:        "*sql.DB",
	readyCheck:      "d.DB.PingContext(ctx)",
	defaultURL:      func(db string) string { return fmt.Sprintf("app:app@tcp(localhost:3306)/%s", db) },
	testURL:         func(db string) string { return fmt.Sprintf("app:app@tcp(localhost:3306)/%s_test", db) },
	composeTemplate: "db/mysql/compose.tmpl",
	ciService: func(testDB, testURL string) string {
		return fmt.Sprintf(`services:
  mysql:
    image: mysql:8.4
    env:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: %s
      MYSQL_USER: app
      MYSQL_PASSWORD: app
    ports:
      - 3306:3306
    options: >-
      --health-cmd "mysqladmin ping -h 127.0.0.1 -uapp -papp"
      --health-interval 2s
      --health-timeout 3s
      --health-retries 30
env:
  TEST_DATABASE_URL: %s
`, testDB, testURL)
	},
	makeTargets: func(db, testURL string) []string {
		return dockerMakeTargets("MySQL",
			fmt.Sprintf(`-docker compose exec -T db mysql -uroot -proot -e "CREATE DATABASE IF NOT EXISTS %[1]s_test; GRANT ALL ON %[1]s_test.* TO 'app'@'%%'"`, db),
			testURL)
	},
}

var sqliteEngine = &engine{
	name:    domain.DatabaseSQLite,
	label:   domain.DatabaseLabel(domain.DatabaseSQLite),
	module:  "modernc.org/sqlite",
	version: "v1.61.0",
	dbFiles: sqlFiles("sqlite"),
	repositoryFiles: []fileTemplate{
		{"db/sql/repository.tmpl", "sqlite.go"},
		{"db/sql/repository_test.tmpl", "sqlite_test.go"},
	},
	dialect: &sqlDialect{
		quote:       func(name string) string { return `"` + name + `"` },
		placeholder: func(int) string { return "?" },
		types: map[domain.FieldType]string{
			domain.FieldString: "TEXT",
			domain.FieldInt:    "INTEGER",
			domain.FieldFloat:  "REAL",
			domain.FieldBool:   "INTEGER",
			domain.FieldTime:   "TEXT",
		},
		idType:    "TEXT",
		timeType:  "TEXT",
		clear:     func(table string) string { return "DELETE FROM " + table },
		textTimes: true,
	},
	repoType:   "SQLiteRepository",
	depsImport: "database/sql",
	depsType:   "*sql.DB",
	readyCheck: "d.DB.PingContext(ctx)",
	defaultURL: func(db string) string { return db + ".db" },
	makeTargets: func(db, _ string) []string {
		return []string{
			"db-reset: ## Delete the SQLite database (it is recreated on the next start)",
			fmt.Sprintf("\trm -f %[1]s.db %[1]s.db-wal %[1]s.db-shm", db),
		}
	},
	gitignore:  []string{"*.db", "*.db-shm", "*.db-wal"},
	dockerNote: "SQLite stores data in a file inside the container, which is lost when the container is replaced. Mount a volume and point DATABASE_URL at it, e.g. `docker run -v ./data:/data -e DATABASE_URL=/data/app.db ...`.",
}

var mongoEngine = &engine{
	name:    domain.DatabaseMongo,
	label:   domain.DatabaseLabel(domain.DatabaseMongo),
	module:  "go.mongodb.org/mongo-driver/v2",
	version: "v2.9.2",
	dbFiles: []fileTemplate{
		{"db/mongo/db.tmpl", "db/db.go"},
		{"db/mongo/db_test.tmpl", "db/db_test.go"},
		{"db/mongo/dbtest.tmpl", "db/dbtest/dbtest.go"},
	},
	repositoryFiles: []fileTemplate{
		{"db/mongo/repository.tmpl", "mongo.go"},
		{"db/mongo/repository_test.tmpl", "mongo_test.go"},
	},
	repoType:        "MongoRepository",
	depsImport:      "go.mongodb.org/mongo-driver/v2/mongo",
	depsType:        "*mongo.Database",
	readyCheck:      "d.DB.Client().Ping(ctx, nil)",
	defaultURL:      func(db string) string { return "mongodb://localhost:27017/" + db },
	testURL:         func(db string) string { return "mongodb://localhost:27017/" + db + "_test" },
	composeTemplate: "db/mongo/compose.tmpl",
	ciService: func(_, testURL string) string {
		return fmt.Sprintf(`services:
  mongo:
    image: mongo:8.0
    env:
      # Lets MongoDB start on the Linux 6.19 to 7.0.13 kernels some runners use
      # (SERVER-121912)
      GLIBC_TUNABLES: glibc.pthread.rseq=1
    ports:
      - 27017:27017
    options: >-
      --health-cmd "mongosh --quiet --eval 'db.adminCommand({ ping: 1 })'"
      --health-interval 2s
      --health-timeout 5s
      --health-retries 30
env:
  TEST_DATABASE_URL: %s
`, testURL)
	},
	makeTargets: func(_, testURL string) []string {
		return dockerMakeTargets("MongoDB", "", testURL)
	},
}

// engines lists the supported databases by name.
var engines = map[string]*engine{
	postgresEngine.name: postgresEngine,
	mysqlEngine.name:    mysqlEngine,
	sqliteEngine.name:   sqliteEngine,
	mongoEngine.name:    mongoEngine,
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
// resource stored in table. Parameters are numbered in the order they appear in
// each statement, because "?" placeholders (MySQL, SQLite) bind by position:
// inserts take the ID, the fields, then the timestamps; updates take the fields,
// updated_at, then the ID.
func addEngineData(data *resourceData, eng *engine, table string) {
	data.Engine = eng.name
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
	data.TextTimes = d.textTimes

	arg := func(goExpr string, isTime bool) string {
		if isTime && d.textTimes {
			return goExpr + ".UTC().Format(timeLayout)"
		}
		return goExpr
	}
	data.CreatedArg, data.UpdatedArg = arg("item.CreatedAt", true), arg("item.UpdatedAt", true)

	columns := []string{data.IDColumn}
	var sets []string
	for i := range data.Fields {
		f := &data.Fields[i]
		f.Column = d.quote(f.JSON)
		f.SQLType = d.types[f.Type]
		f.ArgExpr = arg("item."+f.GoName, f.IsTime)
		f.TextVar = strings.ToLower(f.GoName[:1]) + f.GoName[1:] + "Text"
		columns = append(columns, f.Column)
		sets = append(sets, fmt.Sprintf("%s = %s", f.Column, d.placeholder(i+1)))
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
	n := len(data.Fields)
	data.UpdateQuery = fmt.Sprintf("UPDATE %s SET %s, %s = %s WHERE %s = %s",
		data.Table, strings.Join(sets, ", "), data.UpdatedColumn, d.placeholder(n+1),
		data.IDColumn, d.placeholder(n+2))

	data.CountGo = strconv.Quote("SELECT count(*) FROM " + data.Table)
	for i := range data.Refs {
		ref := &data.Refs[i]
		ref.Column = d.quote(ref.JSON)
		ref.CountGo = strconv.Quote(fmt.Sprintf("SELECT count(*) FROM %s WHERE %s = %s", data.Table, ref.Column, d.placeholder(1)))
	}
	data.ListGo = strconv.Quote(fmt.Sprintf("SELECT %s FROM %s ORDER BY %s, %s LIMIT %s OFFSET %s",
		data.Columns, data.Table, data.CreatedColumn, data.IDColumn, d.placeholder(1), d.placeholder(2)))
	data.GetGo = strconv.Quote(fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s", data.Columns, data.Table, data.IDColumn, d.placeholder(1)))
	data.InsertGo = strconv.Quote(data.InsertQuery)
	data.UpdateGo = strconv.Quote(data.UpdateQuery)
	data.DeleteGo = strconv.Quote(fmt.Sprintf("DELETE FROM %s WHERE %s = %s", data.Table, data.IDColumn, d.placeholder(1)))
	data.ClearGo = strconv.Quote(d.clear(data.Table))
}
