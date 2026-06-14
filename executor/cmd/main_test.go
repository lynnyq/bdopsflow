package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPidFilePath(t *testing.T) {
	path := pidFilePath("test-executor")
	if path == "" {
		t.Fatal("expected non-empty pid file path")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %s", path)
	}
	if filepath.Base(path) != "executor_test-executor.pid" {
		t.Errorf("expected filename executor_test-executor.pid, got %s", filepath.Base(path))
	}
}

func TestPidFilePathDifferentExecutors(t *testing.T) {
	path1 := pidFilePath("executor-1")
	path2 := pidFilePath("executor-2")
	if path1 == path2 {
		t.Errorf("different executor names should produce different paths: %s == %s", path1, path2)
	}
}

func TestAcquireAndReleasePidLock(t *testing.T) {
	f, err := acquirePidLock("test-lock-executor")
	if err != nil {
		t.Fatalf("failed to acquire pid lock: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil file")
	}

	// Verify PID was written
	data, err := os.ReadFile(pidFilePath("test-lock-executor"))
	if err != nil {
		t.Fatalf("failed to read pidfile: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected pidfile to contain PID")
	}

	// Release the lock
	releasePidLock("test-lock-executor", f)

	// Verify pidfile was removed
	if _, err := os.Stat(pidFilePath("test-lock-executor")); !os.IsNotExist(err) {
		t.Error("expected pidfile to be removed after release")
	}
}

func TestAcquirePidLockDuplicate(t *testing.T) {
	f1, err := acquirePidLock("test-dup-executor")
	if err != nil {
		t.Fatalf("failed to acquire first pid lock: %v", err)
	}
	defer releasePidLock("test-dup-executor", f1)

	// Second acquire should fail
	_, err = acquirePidLock("test-dup-executor")
	if err == nil {
		t.Fatal("expected error when acquiring duplicate pid lock")
	}
}

func TestReleasePidLockNilFile(t *testing.T) {
	// Should not panic with nil file
	releasePidLock("test-nil-executor", nil)
}

func TestReleasePidLockNonExistent(t *testing.T) {
	// Should not panic when removing non-existent pidfile
	releasePidLock("non-existent-executor", nil)
}

func TestParseAddrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", nil},
		{"single", "localhost:50051", []string{"localhost:50051"}},
		{"multiple", "a:50051, b:50051, c:50051", []string{"a:50051", "b:50051", "c:50051"}},
		{"with spaces", "  a:50051 , b:50051  ", []string{"a:50051", "b:50051"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAddrs(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d addrs, got %d", len(tt.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %s, got %s", i, tt.expected[i], result[i])
				}
			}
		})
	}
}
