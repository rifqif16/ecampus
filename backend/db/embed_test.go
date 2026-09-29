package dbfs_test

import (
	"io/fs"
	"strings"
	"testing"

	dbfs "github.com/rifqif16/ecampus/backend/db"
)

func TestMigrations_EmbedsSQLFiles(t *testing.T) {
	entries, err := fs.ReadDir(dbfs.Migrations, dbfs.MigrationsDir)
	if err != nil {
		t.Fatalf("read embedded %s: %v", dbfs.MigrationsDir, err)
	}

	var up, down int
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e.Name(), ".up.sql"):
			up++
		case strings.HasSuffix(e.Name(), ".down.sql"):
			down++
		}
	}
	if up == 0 {
		t.Fatal("no .up.sql migrations embedded")
	}
	if up != down {
		t.Fatalf("up migrations = %d, down migrations = %d, want equal", up, down)
	}
}
