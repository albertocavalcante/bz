//go:build !unix

package cache

import "os"

func verificationOpenFlags() int { return os.O_RDONLY }

func verificationDirectoryPath(path string) string { return path }
