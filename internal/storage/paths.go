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

func ItemStagingDir(dataDir, id string) string {
	return filepath.Join(ItemsRoot(dataDir), id+".staging")
}

func ItemBackupDir(dataDir, id string) string {
	return filepath.Join(ItemsRoot(dataDir), id+".backup")
}
