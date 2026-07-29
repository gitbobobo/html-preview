package storage

import (
	"os"
	"path/filepath"
)

const (
	DesktopThumbName = "preview_desktop.webp"
	MobileThumbName  = "preview_mobile.webp"
)

func DesktopThumbPath(dataDir, id string) string {
	return filepath.Join(ItemDir(dataDir, id), DesktopThumbName)
}

func MobileThumbPath(dataDir, id string) string {
	return filepath.Join(ItemDir(dataDir, id), MobileThumbName)
}

func RemoveItemDir(dataDir, id string) error {
	return os.RemoveAll(ItemDir(dataDir, id))
}
