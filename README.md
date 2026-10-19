# locus

A lightweight, zero-configuration, read-only WebDAV file server written in Go.

Designed for quick and secure local network file sharing with native file managers (macOS Finder, Windows Explorer, GNOME Files) and WebDAV clients without risking accidental modifications or deletions.

---

## Features

- **Strict Read-Only Protection**: Blocks mutative WebDAV/HTTP methods (`PUT`, `DELETE`, `MKCOL`, `PROPPATCH`, `MOVE`, `COPY`) and enforces read-only access at the virtual filesystem layer.
- **Smart Noise Filtering**: Automatically hides dotfiles and OS system files (e.g., `.git`, `.DS_Store`, `Thumbs.db`, `desktop.ini`, `$RECYCLE.BIN`, `System Volume Information`) from both directory listings and direct access.
- **Flexible CLI Options**: Supports standard short and long flags (`-d`, `--dir`, `-p`, `--port`, `-P`, `--prefix`, `-h`, `--help`) as well as positional directory arguments.
- **Universal Compatibility**: Works out of the box with native WebDAV clients, mobile apps (Documents, FE File Explorer), and tools like Cyberduck or rclone.
- **Graceful Shutdown**: Intercepts `SIGINT` / `SIGTERM` signals and cleanly terminates active connections.

---

## Installation & Build

Ensure you have [Go](https://go.dev/) (1.20+) installed.

Clone the repository and build the binary:

```bash
git clone https://github.com/itsbycat/locus.git
cd locus
go build -o locus .
```

The resulting executable will be named `locus`.

---

## Usage

### Quick Start

Serve the current working directory on port `8080`:

```bash
./locus
```

Serve a specific directory:

```bash
./locus /path/to/share
# or using flags:
./locus -d /path/to/share
./locus --dir /path/to/share
```

Customize port and mount prefix:

```bash
./locus -p 9000 -P /webdav /path/to/share
# or
./locus --port 9000 --prefix /webdav --dir /path/to/share
```

### Command-Line Options

```text
Usage:
  locus [options] [directory]

Options:
  -d, -dir, --dir <path>
        Directory to serve (default ".")
  -p, -port, --port <port>
        Port to listen on (1-65535, default 8080)
  -P, -pr, -prefix, --prefix <prefix>
        URL prefix to mount WebDAV on (default "/")
  -h, -help, --help
        Show this help message

Examples:
  locus -d ./myfiles -p 8080
  locus --port 9000 --prefix /webdav
  locus /path/to/folder
```

---

## Connecting to WebDAV

When `locus` starts, it outputs the local IP address and port to connect to.

### macOS (Finder)
1. Open **Finder**.
2. Press <kbd>Cmd</kbd> + <kbd>K</kbd> (or select **Go** > **Connect to Server...**).
3. Enter the server address:
   ```text
   http://<YOUR_LOCAL_IP>:8080/
   ```
4. Select **Connect as Guest**.

### Windows (File Explorer)
1. Open **This PC** in File Explorer.
2. Click **Map network drive** or right-click and choose **Add a network location**.
3. Specify the address:
   ```text
   http://<YOUR_LOCAL_IP>:8080/
   ```
4. Complete the wizard without credentials.

### Linux (GNOME Files / Nautilus)
1. Open **Files**.
2. Click **Other Locations** at the bottom of the sidebar.
3. In **Connect to Server**, enter:
   ```text
   dav://<YOUR_LOCAL_IP>:8080/
   ```

---

## Running Tests

Run the unit and integration test suite:

```bash
go test -v ./...
```
