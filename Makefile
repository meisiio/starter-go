.PHONY: run run-provider test test-integration contract-test cover

run:
	go run .

# Listen on all interfaces so another laptop can reach the service (Lab 5, provider side).
run-provider:
	LISTEN_ADDR=0.0.0.0:8086 go run .

test:
	go vet ./...
	go test ./... -count=1

# needs: docker compose up -d postgres and TEST_DATABASE_URL (see docs/TESTING.md)
test-integration:
	go test -run Integration -v -count=1 ./...

# usage: make contract-test NEIGHBOR_BASE_URL=http://<neighbour-ip>:8082
contract-test:
	NEIGHBOR_BASE_URL=$(NEIGHBOR_BASE_URL) go test -run Contract -v -count=1 ./...

cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -n 1
