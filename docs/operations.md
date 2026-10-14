# Baseline operations

Use a local filesystem directory for /data. WAL is unsuitable for SMB/NFS.
On Linux create the bind mount before Compose starts:

```sh
sudo install -d -m 0700 -o 10001 -g 100 ./energything-data
```

For an existing directory, grant UID 10001 access to the directory and existing
database files. Mount ownership overrides image ownership. A read-only or
unwritable directory fails startup with an actionable message before listening.
Windows Docker Desktop maps host permissions differently; validate on the actual
Linux deployment host in task 08.

The application opens SQLite before binding HTTP. Each connection applies
journal_mode=WAL, foreign_keys=1 and busy_timeout=5000. The pool currently has
one connection, establishing a serialized storage seam without application tables.
SIGTERM/interrupt cancels the server context, drains requests within
ENERGYTHING_SHUTDOWN_TIMEOUT, closes HTTP if the timeout expires and closes SQLite
last. A listen failure returns a nonzero process exit.

Keep credentials in private process/Compose configuration. No credentials enter
SQLite, logs or status cards. No provider calls or background jobs exist in task 01.
Do not put runtime files in the image or commit .env, database companions,
exports, private evidence or caches.

Stop the service cleanly before backing up the entire data directory, including
SQLite companions. Do not copy only the main database during live writes.
Restore validation, release CI and actual host acceptance belong to later tasks.

Linux SQLite runtime probes (after cross-compiling storage test executables):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o bin/linux/amd64/storage.test ./internal/storage
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c -o bin/linux/arm64/storage.test ./internal/storage
docker run --rm --platform linux/amd64 --entrypoint /probe -v "$PWD/bin/linux/amd64/storage.test:/probe:ro" energything:local -test.v
docker run --rm --platform linux/arm64 --entrypoint /probe -v "$PWD/bin/linux/arm64/storage.test:/probe:ro" energything:task01-arm64 -test.v
```

The synthetic probe creates tables only in a temporary test database, verifies
pragmas on replacement connections, writes, closes and reopens the same value.
Cross-builds alone do not satisfy the runtime gate.
