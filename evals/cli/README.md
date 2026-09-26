# Native runtime verification

Build with `make release`. Release compilation covers Darwin, Linux and Windows
for ARM64 and AMD64; compilation alone does not prove execution on a target.

## Distribution checks

```sh
go test -race ./...
go vet ./...
python3 -B evals/bootstrap/test_native_packaging.py
python3 -B evals/bootstrap/test_native_installation.py
python3 -B evals/bootstrap/test_bootstrap.py
python3 -B scripts/test_instance.py
```

Python belongs to this distribution's installer and its tests. It
is not part of the installed native runtime. Native installation tests require
a fresh complete release and fail rather than skip when artifacts are stale.

## Linux via Colima / Docker

```sh
docker build -t kos-test-ubuntu:24.04 - < evals/cli/Dockerfile.ubuntu
docker build -t kos-test-fedora:44 - < evals/cli/Dockerfile.fedora
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -o dist/retrieval.test ./internal/retrieval
docker run --rm --network none -v "$PWD/dist:/tests:ro" kos-test-ubuntu:24.04 /tests/retrieval.test -test.v
```

Repeat for Fedora and each internal package. Tests with repository fixtures
need the source checkout mounted read-only at its compile-time absolute path
and the working directory set to the package directory. A Python differential
oracle may skip if its optional Python dependency is absent; native execution
and oracle coverage must be reported separately. Testing two ARM64 distributions
does not certify every distribution or AMD64 execution.

## Windows without a compiler

Crosscompile on the development host:

```sh
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go test -c -o dist/retrieval-tests.exe ./internal/retrieval
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o dist/platformcheck.exe ./tools/platformcheck
```

Transfer these executables and `kos-windows-arm64.exe` to a test directory
in Windows; name the product executable `kos.exe` next to
`platformcheck.exe`. Launch the test executable without flags in PowerShell.
For verbose Go test output quote `'-test.v'` to preserve the native argument.
The platform runner launches the real CLI against synthetic temporary data,
with an empty PATH. It needs no Go, Python or Git installation and deletes its
fixtures. It covers core local operations; Git-dependent handoff and remote
inventory require their external tools and separate execution coverage.

For AMD64, repeat the builds with `GOARCH=amd64` and use the matching product
binary. Windows ARM emulation can exercise this build; report it separately
from native AMD64 hardware.

Execution evidence on 2026-09-24: the user ran the platform runner in the
Windows ARM VM and supplied screenshots showing **13/13 passed** for both
native ARM64 and emulated AMD64, with the external-tool PATH empty. This
covers the core operations listed above. Windows Git-dependent workflows,
the distribution installer and native AMD64 hardware remain unverified.
