.PHONY: test race vet fuzz fmt integration tidy

test:
	go test ./...

race:
	go test -race -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

integration:
	go test -race -count=1 ./...
	go test -C internal/itest -race -count=1 ./...

fuzz:
	go test -fuzz=FuzzEqValue -fuzztime=20s
	go test -fuzz=FuzzIdent -fuzztime=20s

tidy:
	go mod tidy
	go mod tidy -C internal/itest
