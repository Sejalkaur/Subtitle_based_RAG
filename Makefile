.PHONY: all build test run clean ingest search

all: test build

build:
	mkdir -p bin
	go build -o bin/server cmd/server/main.go
	go build -o bin/ingest cmd/ingest/main.go
	go build -o bin/search cmd/search/main.go

test:
	go test -v ./...

run:
	go run cmd/server/main.go

ingest:
	go run cmd/ingest/main.go -mode=all

search:
	go run cmd/search/main.go $(QUERY)

clean:
	rm -rf bin/
