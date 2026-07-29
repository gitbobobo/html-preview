package storage

import (
	"os"
)

// ReplaceItemContent writes content to a staging directory and atomically promotes it
// to the live item directory. On failure the previous live directory is preserved.
func ReplaceItemContent(dataDir, id string, write func(stagingDir string) (int64, error)) (int64, error) {
	stagingDir := ItemStagingDir(dataDir, id)
	if err := os.RemoveAll(stagingDir); err != nil {
		return 0, err
	}

	storedSize, err := write(stagingDir)
	if err != nil {
		os.RemoveAll(stagingDir)
		return 0, err
	}

	if err := promoteStagingToLive(dataDir, id); err != nil {
		os.RemoveAll(stagingDir)
		return 0, err
	}

	return storedSize, nil
}

func promoteStagingToLive(dataDir, id string) error {
	stagingDir := ItemStagingDir(dataDir, id)
	liveDir := ItemDir(dataDir, id)
	backupDir := ItemBackupDir(dataDir, id)

	if err := os.RemoveAll(backupDir); err != nil {
		return err
	}

	if _, err := os.Stat(liveDir); err == nil {
		if err := os.Rename(liveDir, backupDir); err != nil {
			return err
		}
	}

	if err := os.Rename(stagingDir, liveDir); err != nil {
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, liveDir)
		}
		return err
	}

	return nil
}

// RollbackItemContent restores the live directory from backup after a failed metadata update.
func RollbackItemContent(dataDir, id string) error {
	liveDir := ItemDir(dataDir, id)
	backupDir := ItemBackupDir(dataDir, id)

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return nil
	}

	_ = os.RemoveAll(liveDir)
	return os.Rename(backupDir, liveDir)
}

// CleanupItemContentBackup removes the backup directory after a successful replace.
func CleanupItemContentBackup(dataDir, id string) {
	os.RemoveAll(ItemBackupDir(dataDir, id))
}
