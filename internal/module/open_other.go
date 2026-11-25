//go:build !unix

package module

import "os"

func verificationOpenFlags() int { return os.O_RDONLY }

func openForVerification(path string) (*os.File, error) {
	return os.OpenFile(path, verificationOpenFlags(), 0)
}
