.PHONY: build test test-race test-evals release publish test-installer test-platform demo

build:
	CGO_ENABLED=0 go build -trimpath -o dist/kos ./cmd/kos

test:
	go vet ./...
	go test ./...

test-race:
	go test -race ./...

# Synthetic evaluator protocol checks; no agent sessions or consumer vaults.
test-evals:
	python3 -B -m unittest discover -s evals/benchmark -p 'test_*.py'

release:
	go run ./tools/release

# Publish dist/ as the GitHub release v$(VERSION) of rendis/knowledge-os (private), with the rendis account.
VERSION := $(shell cat kernel/VERSION)
publish: release
	@test -z "$$(git status --porcelain --untracked-files=no)" || (echo "commit first: a release is built from a clean commit" >&2; exit 1)
	git tag -f v$(VERSION)
	GH_TOKEN=$$(gh auth token --user rendis) git -c credential.helper= -c 'credential.helper=!gh auth git-credential' push -q origin HEAD:main v$(VERSION)
	GH_TOKEN=$$(gh auth token --user rendis) gh release create v$(VERSION) --repo rendis/knowledge-os --title v$(VERSION) \
	  --notes "kos $(VERSION). Install: curl -fsSL https://github.com/rendis/knowledge-os/releases/latest/download/install-kos.sh | sh" \
	  dist/kos-* dist/SHA256SUMS dist/install-kos.sh dist/install-kos.ps1 dist/VERSION dist/THIRD_PARTY_NOTICES.txt

# Installer and packaging checks; run after `make release`.
test-installer:
	python3 -B evals/bootstrap/test_bootstrap.py
	python3 -B evals/bootstrap/test_integrity.py
	python3 -B evals/bootstrap/test_interactive_onboarding.py < /dev/null
	python3 -B evals/bootstrap/test_native_installation.py
	python3 -B evals/bootstrap/test_vault_catalog.py

# Platform providers against local emulators with the clouds' real CLIs (Docker required).
test-platform:
	python3 -B evals/platform/test_platform_emulators.py

# README recordings from a synthetic demo cell (vhs and jq required).
demo: release
	sh docs/demo/record.sh
