package workspace

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
)

// Usage describes disk usage for a workspace subdirectory.
type Usage struct {
	Name  string
	Bytes int64
}

// Human formats the usage size for terminal display.
func (u Usage) Human() string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(u.Bytes)
	unit := units[0]
	for idx := 0; idx < len(units)-1 && size >= 1024; idx++ {
		size /= 1024
		unit = units[idx+1]
	}
	return fmt.Sprintf("%.1f %s", size, unit)
}

// Status returns usage totals for each workspace subdirectory.
func Status() ([]Usage, error) {
	if err := EnsureLayout(); err != nil {
		return nil, err
	}

	var usages []Usage
	for _, item := range layout {
		path := filepath.Join(Root(), item.name)
		size, err := dirSize(path)
		if err != nil {
			return nil, err
		}
		usages = append(usages, Usage{Name: item.name, Bytes: size})
	}

	sort.Slice(usages, func(i, j int) bool {
		return usages[i].Name < usages[j].Name
	})
	return usages, nil
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk %s: %w", root, err)
	}
	return total, nil
}
