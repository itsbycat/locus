package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Dir    string
	Port   int
	Prefix string
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `locus - lightweight read-only WebDAV server

Usage:
  locus [options] [directory]

Options:
  -d, -dir, --dir <path>
        Directory to serve (default ".")
  -p, -port, --port <port>
        Port to listen on (default 8080)
  -P, -pr, -prefix, --prefix <prefix>
        URL prefix (default "/")
  -h, -help, --help
        Show this help message

Examples:
  locus -d ./myfiles -p 8080
  locus --port 9000 --prefix /webdav
  locus /path/to/folder
`)
}

func LoadConfig() (*Config, error) {
	return ParseConfig(os.Args[1:])
}

func ParseConfig(args []string) (*Config, error) {
	fs := flag.NewFlagSet("locus", flag.ContinueOnError)
	fs.Usage = func() {
		printUsage(fs.Output())
	}

	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-help" || arg == "--h" {
			fs.SetOutput(os.Stdout)
			break
		}
	}

	var dir string
	var port int
	var prefix string

	fs.StringVar(&dir, "dir", ".", "directory to serve")
	fs.StringVar(&dir, "d", ".", "directory to serve (shorthand)")

	fs.IntVar(&port, "port", 8080, "port to listen on")
	fs.IntVar(&port, "p", 8080, "port to listen on (shorthand)")

	fs.StringVar(&prefix, "prefix", "/", "URL prefix")
	fs.StringVar(&prefix, "P", "/", "URL prefix (shorthand)")
	fs.StringVar(&prefix, "pr", "/", "URL prefix (shorthand)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() > 0 && dir == "." {
		dir = fs.Arg(0)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory does not exist: %s", absDir)
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absDir)
	}

	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid port: %d (must be 1-65535)", port)
	}

	cleanPrefix := prefix
	if !strings.HasPrefix(cleanPrefix, "/") {
		cleanPrefix = "/" + cleanPrefix
	}

	return &Config{
		Dir:    absDir,
		Port:   port,
		Prefix: cleanPrefix,
	}, nil
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}
