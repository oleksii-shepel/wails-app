package stream_test

import "os"

func statFile(name string) (any, error) { return os.Stat(name) }
