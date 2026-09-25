.PHONY: build test test-race release test-installer

build:
	CGO_ENABLED=0 go build -trimpath -o dist/vaultctl ./cmd/vaultctl

test:
	go vet ./...
	go test ./...

test-race:
	go test -race ./...

release:
	go run ./tools/release

# Installer and packaging checks; run after `make release`.
test-installer:
	python3 -B scripts/test_instance.py
	python3 -B evals/bootstrap/test_bootstrap.py
	python3 -B evals/bootstrap/test_integrity.py
	python3 -B evals/bootstrap/test_interactive_onboarding.py < /dev/null
	python3 -B evals/bootstrap/test_native_packaging.py
	python3 -B evals/bootstrap/test_native_installation.py
	python3 -B evals/bootstrap/test_vault_catalog.py
