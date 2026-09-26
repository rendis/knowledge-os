# Native executable smoke checks

This development-only runner verifies a compiled `kos` through child processes. Its embedded fixture is synthetic and contains no customer data or host-specific paths. It creates and removes a disposable vault and external cache; the temporary path includes spaces and Unicode. Child processes receive an empty executable search path, so the covered operations cannot depend on Git, Python or Go being installed.

Build on the development host:

```sh
go build -o platformcheck ./tools/platformcheck
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o platformcheck.exe ./tools/platformcheck
```

Place the platform-matching runner beside `kos` (`kos.exe` on Windows) and launch it without arguments. An explicit CLI path remains available:

```sh
./platformcheck --cli /path/to/kos
```

```powershell
.\platformcheck.exe --cli C:\path\to\kos.exe
```

A successful run prints `PASS 13/13 checks` and exits zero. It checks version output, configuration status and resolution, structural audit, links, one Base, SQLite/FTS5 indexing, accent handling, unchanged fingerprints, an edited note, stale content removal, a deleted note and zero results. It rejects nonzero exits, unexpected stderr and mismatched result fields.

This is executable smoke coverage. It does not replace package tests, installer tests, fault-injection tests, benchmarks or Git-dependent handoff/synchronization tests. Cross-compiling the runner is not evidence that it ran in the target OS.
