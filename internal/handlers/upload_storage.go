package handlers

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const publicRootDir = "/app/public"

func safeDeleteUpload(relPath string) error {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" {
		return nil
	}

	cleanRel := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	cleanRel = strings.TrimPrefix(cleanRel, "/")

	if !strings.HasPrefix(cleanRel, "uploads/") {
		return fmt.Errorf("refusing to delete non-upload path: %s", relPath)
	}

	cleanBase := filepath.Clean(publicRootDir)
	targetPath := filepath.Join(cleanBase, filepath.FromSlash(cleanRel))
	cleanTarget := filepath.Clean(targetPath)
	if cleanTarget != cleanBase && !strings.HasPrefix(cleanTarget, cleanBase+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to delete path outside public root: %s", relPath)
	}

	if err := os.Remove(cleanTarget); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return nil
}
