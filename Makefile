build:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -trimpath -o bin/watcher ./cmd/watch
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -trimpath -o bin/untar ./cmd/untar

unit:
	go test -count=1 ./... -vet=off -cover -coverprofile=coverage.out

lint:
	golangci-lint run

generate:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" go generate ./...

.PHONY: run build unit generate lint
