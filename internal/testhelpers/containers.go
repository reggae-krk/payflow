package testhelpers

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type PostgresContainer struct {
	*postgres.PostgresContainer
	ConnectionString string
}

func CreatePostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithInitScripts(allMigrationFiles()...),
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, err
	}
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, err
	}

	return &PostgresContainer{
		PostgresContainer: pgContainer,
		ConnectionString:  connStr,
	}, nil
}

func allMigrationFiles() []string {
	pattern := filepath.Join("..", "..", "migrations", "*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		panic(err)
	}
	sort.Strings(files)
	return files
}