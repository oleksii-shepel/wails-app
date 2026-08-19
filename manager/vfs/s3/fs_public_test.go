package s3

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewAnonymousPublicBucket(t *testing.T) {
	if os.Getenv("WAILS_APP_RUN_NETWORK_TESTS") != "1" {
		t.Skip("set WAILS_APP_RUN_NETWORK_TESTS=1 to run network-backed S3 tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	fs, err := New(ctx, Config{
		Region:    "us-west-2",
		Bucket:    "colorado-public-imagery",
		Prefix:    "DRAPP",
		Anonymous: true,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer fs.Close()

	entries, err := fs.ReadDir(ctx, "")
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry in public bucket prefix")
	}
}