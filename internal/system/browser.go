package system

import "fmt"

// OpenURL opens a URL using the desktop's default handler.
func OpenURL(url string) error {
	if _, err := runner.Run("xdg-open", url); err == nil {
		return nil
	}
	if _, err := runner.Run("gio", "open", url); err == nil {
		return nil
	}
	return fmt.Errorf("open url %s: no supported browser opener found", url)
}
