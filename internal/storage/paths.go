package storage

import (
	"path/filepath"
)

func ItemsRoot(dataDir string) string {
	return filepath.Join(dataDir, "items")
}

func ItemDir(dataDir, id string) string {
	return filepath.Join(ItemsRoot(dataDir), id)
}

// IndexHTMLName is the on-disk filename of an item's rendered HTML page.
const IndexHTMLName = "index.html"

// IndexHTMLPath returns the path of an item's index.html within its item dir.
func IndexHTMLPath(dataDir, id string) string {
	return filepath.Join(ItemDir(dataDir, id), IndexHTMLName)
}

func ItemStagingDir(dataDir, id string) string {
	return filepath.Join(ItemsRoot(dataDir), id+".staging")
}

func ItemBackupDir(dataDir, id string) string {
	return filepath.Join(ItemsRoot(dataDir), id+".backup")
}
