package testfixture

import (
	"context"
	"io/fs"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LoadSchema applies every `*.up.sql` file under fsys/root to pool, in lexical
// order. Use to bootstrap a service schema once per test package, typically
// from the service's embedded migrations:
//
//	import "github.com/paper-board/agents/migrations"
//	testfixture.LoadSchema(t, pool, migrations.SchemaFS, "schema")
//
// LoadSchema does NOT run inside a transaction — migrations may include
// CREATE EXTENSION or other tx-incompatible statements; each file's BEGIN/COMMIT
// is honoured by Postgres at exec time.
func LoadSchema(t *testing.T, pool *pgxpool.Pool, fsys fs.FS, root string) {
	t.Helper()
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		t.Fatalf("testfixture: read dir %q: %v", root, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	ctx := context.Background()
	for _, name := range files {
		b, err := fs.ReadFile(fsys, path.Join(root, name))
		if err != nil {
			t.Fatalf("testfixture: read %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			t.Fatalf("testfixture: apply %s: %v", name, err)
		}
	}
}
