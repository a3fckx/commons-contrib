# commons-contrib

.PHONY: build test run-bounty run-sourcekind run-sim-verifier run-benchmark clean

build:
	go build ./cmd/bounty-scout
	go build ./cmd/sourcekind-persona
	go build ./cmd/sim-verifier
	go build ./cmd/benchmark-run

test:
	go vet ./...
	go build ./...

run-bounty:
	go run ./cmd/bounty-scout

run-sourcekind:
	go run ./cmd/sourcekind-persona

run-sim-verifier:
	go run ./cmd/sim-verifier

run-benchmark:
	go run ./cmd/benchmark-run

clean:
	rm -f bounty-scout sourcekind-persona sim-verifier benchmark-run
