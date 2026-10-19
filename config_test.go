package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigFlags(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		args       []string
		wantDir    string
		wantPort   int
		wantPrefix string
	}{
		{
			name:       "defaults",
			args:       []string{},
			wantDir:    "",
			wantPort:   8080,
			wantPrefix: "/",
		},
		{
			name:       "short flag -d",
			args:       []string{"-d", tempDir},
			wantDir:    tempDir,
			wantPort:   8080,
			wantPrefix: "/",
		},
		{
			name:       "long flag --dir",
			args:       []string{"--dir", tempDir},
			wantDir:    tempDir,
			wantPort:   8080,
			wantPrefix: "/",
		},
		{
			name:       "single dash -dir",
			args:       []string{"-dir", tempDir},
			wantDir:    tempDir,
			wantPort:   8080,
			wantPrefix: "/",
		},
		{
			name:       "short flag -p",
			args:       []string{"-p", "9090"},
			wantPort:   9090,
			wantPrefix: "/",
		},
		{
			name:       "long flag --port",
			args:       []string{"--port", "9090"},
			wantPort:   9090,
			wantPrefix: "/",
		},
		{
			name:       "single dash -port",
			args:       []string{"-port", "9090"},
			wantPort:   9090,
			wantPrefix: "/",
		},
		{
			name:       "short flag -P prefix",
			args:       []string{"-P", "/myfiles"},
			wantPort:   8080,
			wantPrefix: "/myfiles",
		},
		{
			name:       "short flag -pr prefix",
			args:       []string{"-pr", "/myfiles"},
			wantPort:   8080,
			wantPrefix: "/myfiles",
		},
		{
			name:       "long flag --prefix",
			args:       []string{"--prefix", "myfiles"}, // should add leading slash
			wantPort:   8080,
			wantPrefix: "/myfiles",
		},
		{
			name:       "all short flags together",
			args:       []string{"-d", tempDir, "-p", "7070", "-P", "/dav"},
			wantDir:    tempDir,
			wantPort:   7070,
			wantPrefix: "/dav",
		},
		{
			name:       "all long flags together",
			args:       []string{"--dir", tempDir, "--port", "7070", "--prefix", "/dav"},
			wantDir:    tempDir,
			wantPort:   7070,
			wantPrefix: "/dav",
		},
		{
			name:       "positional argument for directory",
			args:       []string{tempDir},
			wantDir:    tempDir,
			wantPort:   8080,
			wantPrefix: "/",
		},
		{
			name:       "positional arg with flags",
			args:       []string{"-p", "3000", tempDir},
			wantDir:    tempDir,
			wantPort:   3000,
			wantPrefix: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantDir != "" && cfg.Dir != tc.wantDir {
				t.Errorf("Dir = %q, want %q", cfg.Dir, tc.wantDir)
			}
			if cfg.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tc.wantPort)
			}
			if cfg.Prefix != tc.wantPrefix {
				t.Errorf("Prefix = %q, want %q", cfg.Prefix, tc.wantPrefix)
			}
		})
	}
}

func TestParseConfigHelp(t *testing.T) {
	helpFlags := [][]string{
		{"-h"},
		{"--h"},
		{"-help"},
		{"--help"},
	}

	for _, args := range helpFlags {
		t.Run(args[0], func(t *testing.T) {
			cfg, err := ParseConfig(args)
			if cfg != nil {
				t.Errorf("expected nil config on %v", args)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Errorf("expected ErrHelp on %v, got %v", args, err)
			}
		})
	}
}

func TestParseConfigErrors(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "nonexistent directory",
			args: []string{"-d", filepath.Join(tempDir, "nonexistent")},
		},
		{
			name: "path is a file not a directory",
			args: []string{"-d", filePath},
		},
		{
			name: "port 0 is invalid",
			args: []string{"-p", "0"},
		},
		{
			name: "negative port is invalid",
			args: []string{"-p", "-1"},
		},
		{
			name: "port > 65535 is invalid",
			args: []string{"-p", "70000"},
		},
		{
			name: "unknown flag",
			args: []string{"--unknown-flag"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(tc.args)
			if err == nil {
				t.Errorf("expected error for %s, got cfg: %+v", tc.name, cfg)
			}
		})
	}
}
