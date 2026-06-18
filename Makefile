# commons-contrib

.PHONY: build test run-bounty run-sourcekind run-sim-verifier clean

build:
	go build ./cmd/bounty-scout
	go build ./cmd/sourcekind-persona
	go build ./cmd/sim-verifier

test:
	go vet ./...
	go build ./...

run-bounty:
	go run ./cmd/bounty-scout

run-sourcekind:
	go run ./cmd/sourcekind-persona

run-sim-verifier:
	go run ./cmd/sim-verifier

clean:
	rm -f bounty-scout sourcekind-persona sim-verifier
