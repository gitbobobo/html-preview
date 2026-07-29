package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	MaxZipBytes        = 20 << 20 // 20MB
	MaxZipDecompressed = 50 << 20 // 50MB
	MaxZipFiles        = 200
	indexHTML          = "index.html"
)

// ZipExtractOpts configures safe zip extraction behavior.
type ZipExtractOpts struct {
	RequireIndexHTML   bool
	MaxZipBytes        int64 // 0 = no archive size limit
	MaxZipDecompressed int64 // 0 = no decompressed size limit
	MaxZipFiles        int   // 0 = no file count limit
}

// UploadZipOpts returns defaults for user-uploaded zip archives.
func UploadZipOpts() ZipExtractOpts {
	return ZipExtractOpts{
		RequireIndexHTML:   true,
		MaxZipBytes:        MaxZipBytes,
		MaxZipDecompressed: MaxZipDecompressed,
		MaxZipFiles:        MaxZipFiles,
	}
}

func validateZipEntryName(name string) error {
	if name == "" {
		return ErrZipEmptyPath
	}
	if strings.Contains(name, "\\") {
		return ErrZipBackslashPath
	}
	if strings.Contains(name, "..") {
		return ErrZipPathTraversal
	}
	if strings.HasPrefix(name, "/") {
		return ErrZipAbsolutePath
	}
	if len(name) >= 2 && name[1] == ':' {
		return ErrZipAbsolutePath
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ErrZipPathTraversal
	}
	return nil
}

func isNestedZip(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".zip")
}

func ExtractZip(dataDir, id string, r io.ReaderAt, size int64) (int64, error) {
	return ExtractZipToDir(ItemDir(dataDir, id), r, size, UploadZipOpts())
}

func ExtractZipToDir(dir string, r io.ReaderAt, size int64, opts ZipExtractOpts) (int64, error) {
	if opts.MaxZipBytes > 0 && size > opts.MaxZipBytes {
		return 0, ErrZipTooLarge
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return 0, ErrZipInvalid
	}

	if opts.MaxZipFiles > 0 && len(zr.File) > opts.MaxZipFiles {
		return 0, ErrZipTooManyFiles
	}

	var totalUncompressed int64
	for _, f := range zr.File {
		if err := validateZipEntryName(f.Name); err != nil {
			return 0, fmt.Errorf("unsafe zip path: %w", err)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return 0, ErrZipSymlink
		}
		if isNestedZip(f.Name) {
			return 0, ErrZipNested
		}
		if f.UncompressedSize64 > 0 {
			totalUncompressed += int64(f.UncompressedSize64)
		} else {
			totalUncompressed += int64(f.UncompressedSize)
		}
		if opts.MaxZipDecompressed > 0 && totalUncompressed > opts.MaxZipDecompressed {
			return 0, ErrZipDecompressedTooLarge
		}
	}

	if opts.RequireIndexHTML {
		hasRootIndex := false
		for _, f := range zr.File {
			if path.Clean(f.Name) == indexHTML {
				hasRootIndex = true
				break
			}
		}
		if !hasRootIndex {
			return 0, ErrZipMissingIndex
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}

	var written int64
	for _, f := range zr.File {
		if err := validateZipEntryName(f.Name); err != nil {
			os.RemoveAll(dir)
			return 0, err
		}
		if f.FileInfo().IsDir() {
			sub := filepathJoin(dir, f.Name)
			if err := os.MkdirAll(sub, 0o755); err != nil {
				os.RemoveAll(dir)
				return 0, err
			}
			continue
		}

		dest := filepathJoin(dir, f.Name)
		if err := os.MkdirAll(path.Dir(dest), 0o755); err != nil {
			os.RemoveAll(dir)
			return 0, err
		}

		rc, err := f.Open()
		if err != nil {
			os.RemoveAll(dir)
			return 0, err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			os.RemoveAll(dir)
			return 0, err
		}

		var limited io.Reader = rc
		if opts.MaxZipDecompressed > 0 {
			limited = io.LimitReader(rc, opts.MaxZipDecompressed+1)
		}
		n, copyErr := io.Copy(out, limited)
		rc.Close()
		out.Close()
		if copyErr != nil {
			os.RemoveAll(dir)
			return 0, copyErr
		}
		written += n
		if opts.MaxZipDecompressed > 0 && written > opts.MaxZipDecompressed {
			os.RemoveAll(dir)
			return 0, ErrZipDecompressedTooLarge
		}
	}

	return size, nil
}

// UnzipArchive extracts a zip file on disk into destDir using the same safety checks as ExtractZipToDir.
func UnzipArchive(srcZipPath, destDir string, opts ZipExtractOpts) (int64, error) {
	f, err := os.Open(srcZipPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return ExtractZipToDir(destDir, f, info.Size(), opts)
}

// filepathJoin joins a filesystem base with a zip-relative path (forward slashes).
func filepathJoin(base, rel string) string {
	return filepath.Join(base, filepath.FromSlash(path.Clean(rel)))
}
