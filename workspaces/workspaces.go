package workspaces

import (
	"os"
)

const (
	defaultDirectoryPermMode = 0755
)

func CreateWorkspaceDirecotry(name, path string) error {
	if err := os.MkdirAll(path, defaultDirectoryPermMode); err != nil {
		return err
	}
	return nil
}
