package migrations

import (
	"embed"
	"io/fs"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

// FS returns the embedded migration filesystem. The files are stored at the
// root of the returned filesystem.
func FS() fs.FS {
	return files
}

// Source creates a golang-migrate source driver that reads the embedded
// migrations.
func Source() (source.Driver, error) {
	return iofs.New(files, ".")
}
