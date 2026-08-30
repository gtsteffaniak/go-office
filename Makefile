.PHONY: test test-integration assets assets-clean

GO_OFFICE_ASSETS ?= $(CURDIR)/assets

test:
	go test ./...

assets:
	go run ./cmd/fetch-assets

assets-clean:
	rm -rf assets

test-integration: assets
	GO_OFFICE_ASSETS=$(GO_OFFICE_ASSETS) go test -tags=integration ./...

# Cross-compile the example server for another GOOS (library is pure Go; assets/x2t stay Linux).
build-example:
	GOOS=$(or $(GOOS),linux) GOARCH=$(or $(GOARCH),amd64) go build -o bin/go-office ./cmd/go-office
