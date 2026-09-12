.PHONY: build build-static test run clean

build:
	mkdir -p dist
	CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor

build-static:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor-static ./cmd/onvif-visor

test:
	gofmt -w cmd internal web
	go test ./...
	go vet ./...

run:
	CGO_ENABLED=1 go run ./cmd/onvif-visor

clean:
	rm -rf dist
