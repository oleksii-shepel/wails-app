package main

import "os"

func statPath(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}
