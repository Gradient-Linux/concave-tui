package docker

import (
	"context"
	"fmt"
	"strings"
)

// PullWithProgress preserves the current image and then pulls the new one.
func PullWithProgress(ctx context.Context, image string, cb func(string)) error {
	return PullWithRollbackSafety(ctx, image, cb)
}

// PullWithRollbackSafety preserves the current image and then pulls the new one.
func PullWithRollbackSafety(ctx context.Context, image string, onProgress func(string)) error {
	if err := TagAsPrevious(image); err != nil {
		return err
	}
	return Pull(ctx, image, onProgress)
}

// ImageExists reports whether an image tag exists locally.
func ImageExists(image string) (bool, error) {
	ctx, cancel := withDefaultTimeout(context.Background())
	defer cancel()

	out, err := commandRunner.RunCommand(ctx, "docker", "image", "inspect", image)
	if err != nil {
		if isMissingImage(err, out) {
			return false, nil
		}
		return false, fmt.Errorf("docker image inspect %s: %w", image, err)
	}
	return true, nil
}

// TagAsPrevious tags an existing image as <repo>:gradient-previous when present.
func TagAsPrevious(image string) error {
	exists, err := ImageExists(image)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	ctx, cancel := withDefaultTimeout(context.Background())
	defer cancel()

	previous := previousImageTag(image)
	if _, err := commandRunner.RunCommand(ctx, "docker", "tag", image, previous); err != nil {
		return fmt.Errorf("docker tag %s %s: %w", image, previous, err)
	}
	return nil
}

// RevertToPrevious retags <repo>:gradient-previous back to the requested image tag.
func RevertToPrevious(image string) error {
	previous := previousImageTag(image)
	exists, err := ImageExists(previous)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("previous image not found: %s", previous)
	}

	ctx, cancel := withDefaultTimeout(context.Background())
	defer cancel()

	if _, err := commandRunner.RunCommand(ctx, "docker", "tag", previous, image); err != nil {
		return fmt.Errorf("docker tag %s %s: %w", previous, image, err)
	}
	return nil
}

func previousImageTag(image string) string {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon] + ":gradient-previous"
	}
	return image + ":gradient-previous"
}

func isMissingImage(err error, out []byte) bool {
	text := strings.ToLower(err.Error() + " " + string(out))
	return strings.Contains(text, "no such image") || strings.Contains(text, "no such object")
}
