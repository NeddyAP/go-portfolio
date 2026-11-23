BINARY := bin/portfolio
LDFLAGS := -s -w

.PHONY: all build build-linux-amd64 build-linux-arm64 run test clean

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/server

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-amd64 ./cmd/server

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY)-linux-arm64 ./cmd/server

run:
	go run ./cmd/server

test:
	go vet ./...
	go test -race -v ./...

clean:
	rm -rf bin/
