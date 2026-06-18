# commons-contrib

.PHONY: build test run-bounty run-sourcekind clean

build:
	go build ./cmd/bounty-scout
	go build ./cmd/sourcekind-persona

test:
	go vet ./...
	go build ./...

run-bounty:
	go run ./cmd/bounty-scout

run-sourcekind:
	go run ./cmd/sourcekind-persona

clean:
	rm -f bounty-scout sourcekind-persona
