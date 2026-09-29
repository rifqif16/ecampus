package db

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const migrateDriverScheme = "pgx5"

var ErrInvalidDatabaseURL = errors.New("invalid database url")

func Migrate(databaseURL string, fsys fs.FS, dir string) (err error) {
	migrateURL, err := toMigrateURL(databaseURL)
	if err != nil {
		return err
	}

	src, err := iofs.New(fsys, dir)
	if err != nil {
		return fmt.Errorf("open migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		return errors.Join(fmt.Errorf("init migrator: %w", err), src.Close())
	}
	defer func() {
		srcErr, dbErr := m.Close()
		err = errors.Join(err, srcErr, dbErr)
	}()

	if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", upErr)
	}
	return nil
}

func toMigrateURL(databaseURL string) (string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("%w: malformed url", ErrInvalidDatabaseURL)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("%w: unsupported scheme %q", ErrInvalidDatabaseURL, u.Scheme)
	}

	u.Scheme = migrateDriverScheme
	return u.String(), nil
}
