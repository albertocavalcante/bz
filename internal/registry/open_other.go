//go:build !unix

package registry

import "os"

func registryReadFlags() int { return os.O_RDONLY }

func registryDirectoryPath(path string) string { return path }
