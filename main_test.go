package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv_BasicKeyValue(t *testing.T) {
	f := writeTempEnv(t, "DTRON_TEST_KEY=hello\n")
	os.Unsetenv("DTRON_TEST_KEY")
	defer os.Unsetenv("DTRON_TEST_KEY")

	loadDotEnv(f)
	if got := os.Getenv("DTRON_TEST_KEY"); got != "hello" {
		t.Errorf("DTRON_TEST_KEY = %q, want %q", got, "hello")
	}
}

func TestLoadDotEnv_StripsDoubleQuotes(t *testing.T) {
	f := writeTempEnv(t, `DTRON_TEST_QUOTED="quoted value"`+"\n")
	os.Unsetenv("DTRON_TEST_QUOTED")
	defer os.Unsetenv("DTRON_TEST_QUOTED")

	loadDotEnv(f)
	if got := os.Getenv("DTRON_TEST_QUOTED"); got != "quoted value" {
		t.Errorf("got %q, want %q", got, "quoted value")
	}
}

func TestLoadDotEnv_StripsSingleQuotes(t *testing.T) {
	f := writeTempEnv(t, "DTRON_TEST_SQ='single quoted'\n")
	os.Unsetenv("DTRON_TEST_SQ")
	defer os.Unsetenv("DTRON_TEST_SQ")

	loadDotEnv(f)
	if got := os.Getenv("DTRON_TEST_SQ"); got != "single quoted" {
		t.Errorf("got %q, want %q", got, "single quoted")
	}
}

func TestLoadDotEnv_IgnoresComments(t *testing.T) {
	f := writeTempEnv(t, "# this is a comment\nDTRON_TEST_CMT=value\n")
	os.Unsetenv("DTRON_TEST_CMT")
	defer os.Unsetenv("DTRON_TEST_CMT")

	loadDotEnv(f)
	if got := os.Getenv("DTRON_TEST_CMT"); got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

func TestLoadDotEnv_IgnoresBlankLines(t *testing.T) {
	f := writeTempEnv(t, "\n\nDTRON_TEST_BLANK=ok\n\n")
	os.Unsetenv("DTRON_TEST_BLANK")
	defer os.Unsetenv("DTRON_TEST_BLANK")

	loadDotEnv(f)
	if got := os.Getenv("DTRON_TEST_BLANK"); got != "ok" {
		t.Errorf("got %q, want %q", got, "ok")
	}
}

func TestLoadDotEnv_DoesNotOverwriteExisting(t *testing.T) {
	f := writeTempEnv(t, "DTRON_TEST_EXISTING=from_file\n")
	_ = os.Setenv("DTRON_TEST_EXISTING", "from_env")
	defer os.Unsetenv("DTRON_TEST_EXISTING")

	loadDotEnv(f)
	if got := os.Getenv("DTRON_TEST_EXISTING"); got != "from_env" {
		t.Errorf("shell value was overwritten: got %q, want %q", got, "from_env")
	}
}

func TestLoadDotEnv_MissingFileIsNoop(_ *testing.T) {
	// Should not panic or error on a missing .env file.
	loadDotEnv("/tmp/definitely_does_not_exist_dtron_test.env")
}

// writeTempEnv writes content to a temp file and returns its path.
func writeTempEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write temp env: %v", err)
	}
	return path
}
