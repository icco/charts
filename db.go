package graphql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/GuiaBolso/darwin"
	"github.com/opencensus-integrations/ocsql"

	// Needed to talk to postgres
	_ "github.com/lib/pq"
)

var (
	db         *sql.DB
	driver     = "postgres"
	migrations = []darwin.Migration{
		{
			Version:     1,
			Description: "Creating table posts",
			Script: `
      CREATE TABLE graphs (
        id serial primary key,
        description text,
        creator_id text,
        data jsonb,
        created_at timestamp with time zone,
        modified_at timestamp with time zone
      );
      `,
		},
		{
			Version:     2,
			Description: "Create users",
			Script: `
      CREATE TABLE users(
        id text primary key,
        role text,
        apikey UUID DEFAULT gen_random_uuid(),
        created_at timestamp with time zone,
        modified_at timestamp with time zone
      );
      `,
		},
	}
)

// InitDB creates a package global db connection from a database string.
func InitDB(dataSourceName string) (*sql.DB, error) {
	var err error

	// Connect to Database
	wrappedDriver, err := ocsql.Register(driver, ocsql.WithAllTraceOptions())
	if err != nil {
		return nil, fmt.Errorf("Failed to register the ocsql driver: %v", err)
	}

	db, _ = sql.Open(wrappedDriver, dataSourceName)
	if err = db.PingContext(context.Background()); err != nil {
		return nil, err
	}

	// Migrate
	driver := darwin.NewGenericDriver(db, darwin.PostgresDialect{})
	d := darwin.New(driver, migrations, nil)
	err = d.Migrate()
	if err != nil {
		return nil, err
	}

	return db, err
}
