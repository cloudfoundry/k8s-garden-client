unit:
	go test -count=1 ./... -vet=off -cover -coverprofile=coverage.out

lint:
	golangci-lint run

generate:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" go generate ./...

.PHONY: run build unit generate lint
