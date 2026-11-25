//go:build !unix

package cache

import "os"

func verificationOpenFlags() int { return os.O_RDONLY }
