package migrations

import (
	"errors"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// TestEmbeddedMigrationsParse 确保内嵌的 SQL 文件是合法的
// golang-migrate source（0001_init、0002_builtin_templates、
// 0003_markdown_templates、0004_channel_templates、
// 0005_channel_payload_templates）
func TestEmbeddedMigrationsParse(t *testing.T) {
	src, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatalf("iofs.New: %v", err)
	}
	defer src.Close()

	v1, err := src.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("first migration version = %d, want 1", v1)
	}
	v2, err := src.Next(v1)
	if err != nil {
		t.Fatalf("Next(1): %v", err)
	}
	if v2 != 2 {
		t.Fatalf("second migration version = %d, want 2", v2)
	}
	v3, err := src.Next(v2)
	if err != nil {
		t.Fatalf("Next(2): %v", err)
	}
	if v3 != 3 {
		t.Fatalf("third migration version = %d, want 3", v3)
	}
	v4, err := src.Next(v3)
	if err != nil {
		t.Fatalf("Next(3): %v", err)
	}
	if v4 != 4 {
		t.Fatalf("fourth migration version = %d, want 4", v4)
	}
	v5, err := src.Next(v4)
	if err != nil {
		t.Fatalf("Next(4): %v", err)
	}
	if v5 != 5 {
		t.Fatalf("fifth migration version = %d, want 5", v5)
	}
	if _, err := src.Next(v5); err == nil {
		t.Fatal("expected exactly five migrations")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Next(5) err = %v, want os.ErrNotExist", err)
	}

	for _, v := range []uint{v1, v2, v3, v4, v5} {
		up, _, err := src.ReadUp(v)
		if err != nil {
			t.Fatalf("ReadUp(%d): %v", v, err)
		}
		up.Close()
		down, _, err := src.ReadDown(v)
		if err != nil {
			t.Fatalf("ReadDown(%d): %v", v, err)
		}
		down.Close()
	}
}
