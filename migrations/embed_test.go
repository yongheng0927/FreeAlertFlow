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
// 0005_channel_payload_templates、0006_oauth_states、0007_app_settings、
// 0008_login_attempts、0009_channel_plaintext_credentials、
// 0010_user_bootstrap）
func TestEmbeddedMigrationsParse(t *testing.T) {
	src, err := iofs.New(FS, ".")
	if err != nil {
		t.Fatalf("iofs.New: %v", err)
	}
	defer src.Close()

	versions := []uint{1}
	for i := uint(2); i <= 10; i++ {
		versions = append(versions, i)
	}
	v, err := src.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	for i, want := range versions {
		if v != want {
			t.Fatalf("migration %d version = %d, want %d", i, v, want)
		}
		if i < len(versions)-1 {
			if v, err = src.Next(v); err != nil {
				t.Fatalf("Next(%d): %v", v, err)
			}
		}
	}
	if _, err := src.Next(10); err == nil {
		t.Fatal("expected exactly ten migrations")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Next(10) err = %v, want os.ErrNotExist", err)
	}

	for _, v := range versions {
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
