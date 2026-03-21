package workspace

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backup archives notebooks and models into ~/gradient/backups.
func Backup() (string, error) {
	if err := EnsureLayout(); err != nil {
		return "", err
	}

	name := "workspace-backup-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	target := filepath.Join(Root(), "backups", name)

	file, err := os.Create(target)
	if err != nil {
		return "", fmt.Errorf("create backup %s: %w", target, err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, dir := range []string{"models", "notebooks"} {
		src := filepath.Join(Root(), dir)
		if err := addDirToArchive(tw, src, dir); err != nil {
			return "", err
		}
	}

	return target, nil
}

func addDirToArchive(tw *tar.Writer, srcRoot, archiveRoot string) error {
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("header %s: %w", path, err)
		}

		rel := strings.TrimPrefix(path, srcRoot)
		rel = strings.TrimPrefix(rel, string(filepath.Separator))
		header.Name = filepath.ToSlash(filepath.Join(archiveRoot, rel))
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write header %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer file.Close()

		if _, err := io.Copy(tw, file); err != nil {
			return fmt.Errorf("copy %s: %w", path, err)
		}
		return nil
	})
}
