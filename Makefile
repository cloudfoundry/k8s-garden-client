
KIND ?= kind
GOVERSION ?= $(shell go version | awk '{print $$3}' | sed 's/\.[0-9]*$$//')
KIND_CLUSTER ?= cfk8s

build:
	@mkdir -p bin
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -trimpath -o bin/rep ./cmd/rep
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -trimpath -o bin/watcher ./cmd/watch
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -trimpath -o bin/untar ./cmd/untar

image:
	docker build -t k8s-rep:latest .

unit:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" go test -count=1 ./... -vet=off -cover -coverprofile=coverage.out

lint:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" golangci-lint run

generate:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" go generate ./...

kind:
	rm -rf kind && git clone https://github.com/cloudfoundry/kind-deployment.git kind
	make -C kind create-kind

delete-kind:
	make -C kind down

load-kind: image
	$(KIND) load docker-image k8s-rep:latest --name $(KIND_CLUSTER)
	
install:
	yq -e -i '.image.repository = "k8s-rep"' helm/values.yaml
	yq -e -i '.image.tag = "latest"' helm/values.yaml
	yq -e -i '.charts.k8sRep.url = strenv(PWD) + "/helm"' kind/versions.yaml

	make -C kind init install login bootstrap-complete

integration: kind load-kind install
	make -C kind smoke

.PHONY: run build image integration unit generate lint certs load-kind install kind delete-kind
