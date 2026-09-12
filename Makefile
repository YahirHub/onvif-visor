.PHONY: build test run clean

build:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/onvif-visor ./cmd/onvif-visor

test:
	gofmt -w cmd internal web
	go test ./...
	go vet ./...

run:
	go run ./cmd/onvif-visor

clean:
	rm -rf dist
