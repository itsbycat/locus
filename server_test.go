package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/webdav"
)

func TestIsHiddenOrSystemName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"", false},
		{".", false},
		{"..", false},
		{"/", false},
		{"normal.txt", false},
		{"regular_dir", false},
		{"my.file.tar.gz", false},
		{".git", true},
		{".gitignore", true},
		{".DS_Store", true},
		{".env", true},
		{"thumbs.db", true},
		{"Thumbs.db", true},
		{"THUMBS.DB", true},
		{"desktop.ini", true},
		{"Desktop.ini", true},
		{"$RECYCLE.BIN", true},
		{"$recycle.bin", true},
		{"system volume information", true},
		{"System Volume Information", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isHiddenOrSystemName(tc.name)
			if got != tc.expected {
				t.Errorf("isHiddenOrSystemName(%q) = %v, expected %v", tc.name, got, tc.expected)
			}
		})
	}
}

func TestIsHiddenOrSystemPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"", false},
		{"/", false},
		{".", false},
		{"/a/b/c", false},
		{"a/b/c.txt", false},
		{"/a/.hidden/c", true},
		{"/.git", true},
		{".git", true},
		{"/a/b/.DS_Store", true},
		{"/path/to/Thumbs.db", true},
		{"path/to/desktop.ini", true},
		{"/path/$RECYCLE.BIN/file", true},
		{"path\\to\\System Volume Information\\file", true},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := isHiddenOrSystemPath(tc.path)
			if got != tc.expected {
				t.Errorf("isHiddenOrSystemPath(%q) = %v, expected %v", tc.path, got, tc.expected)
			}
		})
	}
}

func TestReadOnlyFSMutativeMethods(t *testing.T) {
	tempDir := t.TempDir()
	fs := ReadOnlyFS{Dir: webdav.Dir(tempDir)}
	ctx := context.Background()

	if err := fs.Mkdir(ctx, "testdir", 0755); !errors.Is(err, os.ErrPermission) {
		t.Errorf("Mkdir expected ErrPermission, got %v", err)
	}

	if err := fs.RemoveAll(ctx, "testdir"); !errors.Is(err, os.ErrPermission) {
		t.Errorf("RemoveAll expected ErrPermission, got %v", err)
	}

	if err := fs.Rename(ctx, "old", "new"); !errors.Is(err, os.ErrPermission) {
		t.Errorf("Rename expected ErrPermission, got %v", err)
	}
}

func TestReadOnlyFSStatAndOpenFile(t *testing.T) {
	tempDir := t.TempDir()
	visibleFile := filepath.Join(tempDir, "hello.txt")
	hiddenFile := filepath.Join(tempDir, ".hidden.txt")
	systemFile := filepath.Join(tempDir, "Thumbs.db")

	if err := os.WriteFile(visibleFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create visible file: %v", err)
	}
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("failed to create hidden file: %v", err)
	}
	if err := os.WriteFile(systemFile, []byte("system"), 0644); err != nil {
		t.Fatalf("failed to create system file: %v", err)
	}

	fs := ReadOnlyFS{Dir: webdav.Dir(tempDir)}
	ctx := context.Background()

	// Stat tests
	if _, err := fs.Stat(ctx, "/hello.txt"); err != nil {
		t.Errorf("Stat on visible file failed: %v", err)
	}
	if _, err := fs.Stat(ctx, "/.hidden.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat on hidden file expected ErrNotExist, got %v", err)
	}
	if _, err := fs.Stat(ctx, "/Thumbs.db"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat on system file expected ErrNotExist, got %v", err)
	}

	// OpenFile write flags tests
	writeFlags := []int{
		os.O_WRONLY,
		os.O_RDWR,
		os.O_APPEND,
		os.O_CREATE,
		os.O_TRUNC,
		os.O_RDWR | os.O_CREATE,
	}
	for _, flag := range writeFlags {
		if _, err := fs.OpenFile(ctx, "/hello.txt", flag, 0644); !errors.Is(err, os.ErrPermission) {
			t.Errorf("OpenFile with flag %d expected ErrPermission, got %v", flag, err)
		}
	}

	// OpenFile hidden/system tests
	if _, err := fs.OpenFile(ctx, "/.hidden.txt", os.O_RDONLY, 0); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("OpenFile on hidden file expected ErrNotExist, got %v", err)
	}
	if _, err := fs.OpenFile(ctx, "/Thumbs.db", os.O_RDONLY, 0); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("OpenFile on system file expected ErrNotExist, got %v", err)
	}

	// OpenFile read-only on visible file
	f, err := fs.OpenFile(ctx, "/hello.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile on visible file failed: %v", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("failed to read from opened file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(content))
	}
}

func TestFilteredFileReaddir(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	filesToCreate := []string{
		"alpha.txt",
		"beta.txt",
		".hidden",
		".DS_Store",
		"Thumbs.db",
		"desktop.ini",
	}
	for _, f := range filesToCreate {
		if err := os.WriteFile(filepath.Join(tempDir, f), []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	fs := ReadOnlyFS{Dir: webdav.Dir(tempDir)}
	ctx := context.Background()

	// Test Readdir(0)
	dirFile, err := fs.OpenFile(ctx, "/", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("failed to open dir: %v", err)
	}
	defer dirFile.Close()

	entries, err := dirFile.Readdir(0)
	if err != nil {
		t.Fatalf("Readdir(0) failed: %v", err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(names), names)
	}
	for _, name := range names {
		if name != "alpha.txt" && name != "beta.txt" {
			t.Errorf("unexpected entry in directory listing: %s", name)
		}
	}

	// Test incremental Readdir(count > 0)
	dirFile2, err := fs.OpenFile(ctx, "/", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("failed to open dir for incremental readdir: %v", err)
	}
	defer dirFile2.Close()

	batch1, err := dirFile2.Readdir(1)
	if err != nil {
		t.Fatalf("Readdir(1) batch 1 failed: %v", err)
	}
	if len(batch1) != 1 {
		t.Fatalf("expected 1 entry in batch 1, got %d", len(batch1))
	}

	batch2, err := dirFile2.Readdir(1)
	if err != nil {
		t.Fatalf("Readdir(1) batch 2 failed: %v", err)
	}
	if len(batch2) != 1 {
		t.Fatalf("expected 1 entry in batch 2, got %d", len(batch2))
	}

	if batch1[0].Name() == batch2[0].Name() {
		t.Errorf("batch 1 and batch 2 returned the same entry: %s", batch1[0].Name())
	}

	// End of directory
	_, err = dirFile2.Readdir(1)
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected EOF at end of directory, got %v", err)
	}
}

func TestWebDAVIntegration(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "doc.txt"), []byte("locus doc"), 0644); err != nil {
		t.Fatalf("failed to create doc.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".secret"), []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to create .secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "Thumbs.db"), []byte("cache"), 0644); err != nil {
		t.Fatalf("failed to create Thumbs.db: %v", err)
	}

	cfg := &Config{
		Dir:    tempDir,
		Port:   8080,
		Prefix: "/",
	}
	srv := NewServer(cfg)
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	client := ts.Client()

	// 1. OPTIONS request
	optReq, err := http.NewRequest("OPTIONS", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create OPTIONS request: %v", err)
	}
	optResp, err := client.Do(optReq)
	if err != nil {
		t.Fatalf("OPTIONS failed: %v", err)
	}
	defer optResp.Body.Close()
	if optResp.StatusCode != http.StatusOK {
		t.Errorf("OPTIONS expected 200, got %d", optResp.StatusCode)
	}
	if optResp.Header.Get("DAV") == "" {
		t.Errorf("OPTIONS response missing DAV header")
	}

	// 2. PROPFIND on root collection
	propReq, err := http.NewRequest("PROPFIND", ts.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create PROPFIND request: %v", err)
	}
	propReq.Header.Set("Depth", "1")
	propResp, err := client.Do(propReq)
	if err != nil {
		t.Fatalf("PROPFIND failed: %v", err)
	}
	defer propResp.Body.Close()
	if propResp.StatusCode != http.StatusMultiStatus {
		t.Errorf("PROPFIND expected 207 Multi-Status, got %d", propResp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(propResp.Body)
	if err != nil {
		t.Fatalf("failed to read PROPFIND body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "doc.txt") {
		t.Errorf("PROPFIND response should contain doc.txt: %s", body)
	}
	if strings.Contains(body, ".secret") {
		t.Errorf("PROPFIND response should NOT contain .secret: %s", body)
	}
	if strings.Contains(body, "Thumbs.db") {
		t.Errorf("PROPFIND response should NOT contain Thumbs.db: %s", body)
	}

	// 3. GET on visible file
	getResp, err := client.Get(ts.URL + "/doc.txt")
	if err != nil {
		t.Fatalf("GET /doc.txt failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("GET /doc.txt expected 200, got %d", getResp.StatusCode)
	}
	content, _ := io.ReadAll(getResp.Body)
	if string(content) != "locus doc" {
		t.Errorf("GET /doc.txt content expected 'locus doc', got %q", string(content))
	}

	// 4. GET on hidden file -> 404
	getHiddenResp, err := client.Get(ts.URL + "/.secret")
	if err != nil {
		t.Fatalf("GET /.secret failed: %v", err)
	}
	defer getHiddenResp.Body.Close()
	if getHiddenResp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /.secret expected 404, got %d", getHiddenResp.StatusCode)
	}

	// 5. GET on system file -> 404
	getSysResp, err := client.Get(ts.URL + "/Thumbs.db")
	if err != nil {
		t.Fatalf("GET /Thumbs.db failed: %v", err)
	}
	defer getSysResp.Body.Close()
	if getSysResp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /Thumbs.db expected 404, got %d", getSysResp.StatusCode)
	}

	// 6. Mutating requests should be rejected with 405 Method Not Allowed
	putReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/new.txt", strings.NewReader("write"))
	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("PUT expected 405 Method Not Allowed, got %d", putResp.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/doc.txt", nil)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE failed: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("DELETE expected 405 Method Not Allowed, got %d", delResp.StatusCode)
	}

	mkcolReq, _ := http.NewRequest("MKCOL", ts.URL+"/newdir", nil)
	mkcolResp, err := client.Do(mkcolReq)
	if err != nil {
		t.Fatalf("MKCOL failed: %v", err)
	}
	defer mkcolResp.Body.Close()
	if mkcolResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("MKCOL expected 405 Method Not Allowed, got %d", mkcolResp.StatusCode)
	}
}

func TestWebDAVPrefixMount(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "shared.txt"), []byte("shared content"), 0644); err != nil {
		t.Fatalf("failed to create shared.txt: %v", err)
	}

	cfg := &Config{
		Dir:    tempDir,
		Port:   8080,
		Prefix: "/webdav",
	}
	srv := NewServer(cfg)
	ts := httptest.NewServer(srv.httpServer.Handler)
	defer ts.Close()

	client := ts.Client()

	// GET on prefix subpath
	getResp, err := client.Get(ts.URL + "/webdav/shared.txt")
	if err != nil {
		t.Fatalf("GET /webdav/shared.txt failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("GET /webdav/shared.txt expected 200, got %d", getResp.StatusCode)
	}

	// PROPFIND on bare prefix
	propReq, err := http.NewRequest("PROPFIND", ts.URL+"/webdav", nil)
	if err != nil {
		t.Fatalf("failed to create PROPFIND request: %v", err)
	}
	propResp, err := client.Do(propReq)
	if err != nil {
		t.Fatalf("PROPFIND failed: %v", err)
	}
	defer propResp.Body.Close()
	if propResp.StatusCode != http.StatusMultiStatus {
		t.Errorf("PROPFIND /webdav expected 207 Multi-Status, got %d", propResp.StatusCode)
	}
}
