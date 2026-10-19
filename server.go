package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

func isHiddenOrSystemName(name string) bool {
	if name == "" || name == "." || name == ".." || name == "/" {
		return false
	}

	if strings.HasPrefix(name, ".") {
		return true
	}

	systemFiles := []string{
		"thumbs.db",
		"desktop.ini",
		"$RECYCLE.BIN",
		"system volume information",
	}

	for _, sysName := range systemFiles {
		if strings.EqualFold(name, sysName) {
			return true
		}
	}

	return false
}

func isHiddenOrSystemPath(p string) bool {
	cleanPath := strings.ReplaceAll(filepath.ToSlash(p), "\\", "/")
	parts := strings.Split(cleanPath, "/")

	for _, part := range parts {
		if isHiddenOrSystemName(part) {
			return true
		}
	}
	return false
}

type filteredFile struct {
	webdav.File
}

func (f filteredFile) Readdir(count int) ([]os.FileInfo, error) {
	if count <= 0 {
		infos, err := f.File.Readdir(count)
		if err != nil {
			return nil, err
		}
		filtered := make([]os.FileInfo, 0, len(infos))
		for _, info := range infos {
			if !isHiddenOrSystemName(info.Name()) {
				filtered = append(filtered, info)
			}
		}
		return filtered, nil
	}

	var filtered []os.FileInfo
	for len(filtered) < count {
		infos, err := f.File.Readdir(count - len(filtered))
		for _, info := range infos {
			if !isHiddenOrSystemName(info.Name()) {
				filtered = append(filtered, info)
				if len(filtered) == count {
					break
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) && len(filtered) > 0 {
				return filtered, nil
			}
			return filtered, err
		}
		if len(infos) == 0 {
			break
		}
	}
	return filtered, nil
}

type ReadOnlyFS struct {
	Dir webdav.Dir
}

func (fs ReadOnlyFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return os.ErrPermission
}

func (fs ReadOnlyFS) RemoveAll(ctx context.Context, name string) error {
	return os.ErrPermission
}

func (fs ReadOnlyFS) Rename(ctx context.Context, oldName, newName string) error {
	return os.ErrPermission
}

func (fs ReadOnlyFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if isHiddenOrSystemPath(name) {
		return nil, os.ErrNotExist
	}
	return fs.Dir.Stat(ctx, name)
}

func (fs ReadOnlyFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if isHiddenOrSystemPath(name) {
		return nil, os.ErrNotExist
	}
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, os.ErrPermission
	}

	f, err := fs.Dir.OpenFile(ctx, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	return filteredFile{File: f}, nil
}

type Server struct {
	cfg        *Config
	httpServer *http.Server
}

func NewServer(cfg *Config) *Server {
	prefix := strings.TrimRight(cfg.Prefix, "/")
	wdHandler := &webdav.Handler{
		Prefix:     prefix,
		FileSystem: ReadOnlyFS{Dir: webdav.Dir(cfg.Dir)},
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil && !os.IsNotExist(err) && !os.IsPermission(err) {
				log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
			}
		},
	}

	mux := http.NewServeMux()
	handler := readOnlyMiddleware(wdHandler)
	if prefix == "" {
		mux.Handle("/", handler)
	} else {
		mux.Handle(prefix, handler)
		mux.Handle(prefix+"/", handler)
	}

	return &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.Port),
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

func readOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND", "LOCK", "UNLOCK":
			log.Printf("%s %s: %v", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		default:
			log.Printf("%s %s: %v", r.Method, r.URL.Path, r.RemoteAddr)
			w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND, LOCK, UNLOCK")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
