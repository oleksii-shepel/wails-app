package archive

import (
	"errors"
	"io"
	"os"
)

// PublishNewArchive copies a completed archive into a new destination. The
// exclusive open protects files created after the UI's availability check.
func PublishNewArchive(tempPath, destination string) error {
	source, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(target, source)
	if err := errors.Join(copyErr, target.Close()); err != nil {
		return errors.Join(err, os.Remove(destination))
	}
	return nil
}
