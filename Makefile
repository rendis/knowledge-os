.PHONY: build test test-race release test-legacy
build:
	CGO_ENABLED=0 go build -trimpath -o dist/vaultctl ./cmd/vaultctl

test:
	go test ./...

test-race:
	go test -race ./...

release:
	go run ./tools/release

test-legacy:
	python3 -B evals/bootstrap/test_bootstrap.py
	python3 -B kernel/90-Meta/test_instance.py
