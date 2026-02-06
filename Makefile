
KIND ?= kind
GOVERSION ?= $(shell go version | awk '{print $$3}' | sed 's/\.[0-9]*$$//')
KIND_CLUSTER ?= k8s-rep

build:
	@mkdir -p bin
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)"  GOTOOLCHAIN=$(GOVERSION).0+auto CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -o bin/rep ./cmd/rep
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)"  GOTOOLCHAIN=$(GOVERSION).0+auto CGO_ENABLED=0 go build -ldflags "-w -s" -trimpath -o bin/watcher ./cmd/watch

image:
	docker build -t k8s-rep:latest .

unit:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" GOTOOLCHAIN=$(GOVERSION).0+auto go test -count=1 ./... -vet=off -cover -coverprofile=coverage.out -args --ginkgo.label-filter=!integration

lint:
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" GOTOOLCHAIN=$(GOVERSION).0+auto golangci-lint run

generate:
	go generate ./...

kind:
	$(KIND) create cluster --name $(KIND_CLUSTER) --config="./integration/assets/values-files/kind.yaml"

delete-kind:
	$(KIND) delete cluster --name $(KIND_CLUSTER)
	rm -rf certs

load-kind: image
	$(KIND) load docker-image k8s-rep:latest --name $(KIND_CLUSTER)

install: certs
	kubectl create namespace cf-workloads --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic cert --from-file=tls.crt=./certs/tls.crt --from-file=tls.key=./certs/tls.key --from-file=ca.crt=./certs/ca.crt -n default --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic postgres-tls --from-file=tls.crt=./certs/tls.crt --from-file=tls.key=./certs/tls.key --from-file=ca.crt=./certs/ca.crt -n default --dry-run=client -o yaml | kubectl apply -f -
	kubectl create secret generic instance-identity --from-file=tls.crt=./certs/ca.crt --from-file=tls.key=./certs/ca.key -n default --dry-run=client -o yaml | kubectl apply -f -
	kubectl create configmap postgres-init-scripts --from-file=./integration/assets/db-init-scripts/ -n default --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.1/standard-install.yaml
	helm upgrade --hide-notes --install postgres --repo https://charts.bitnami.com/bitnami postgresql --values ./integration/assets/values-files/postgres.yaml --wait --namespace default
	helm upgrade --install loggregator-agent oci://ghcr.io/cloudfoundry/helm/loggregator-agent --set "forwarderAgent.enabled=true" --set forwarderAgent.certificateSecret=cert --set loggregatorAgent.certificateSecret=cert --set syslogAgent.certificateSecret=cert --wait
	helm upgrade --install diego oci://ghcr.io/cloudfoundry/helm/diego --set "auctioneer.enabled=true" --set "bbs.enabled=true" --set "fileserver.enabled=true" --set dbPassword=postgres --set bbs.certificateSecret=cert --set auctioneer.certificateSecret=cert --wait
	helm upgrade --install dev ./helm --set image.repository=k8s-rep --set image.tag=latest --set-file caCertificate=./certs/ca.crt --set loggregatorCertificateSecret=cert --set "nodeSelector=" --wait --namespace default
	kubectl wait --for=condition=Available --timeout=120s deployment/bbs -n default

certs:
	mkdir -p certs
	openssl genrsa -traditional -out certs/ca.key 4096
	openssl req -x509 -key ./certs/ca.key -out certs/ca.crt -days 365 -nodes -subj "/CN=ca/O=ca" > /dev/null 2>&1

	openssl genrsa -traditional -out certs/tls.key 2048
	openssl req -new -key ./certs/tls.key -out ./certs/tls.csr -nodes -subj "/CN=k8s-rep-integration/O=k8s-rep-integration" > /dev/null 2>&1
	echo "subjectAltName=DNS:*.default.svc.cluster.local,DNS:metron,DNS:localhost,IP:127.0.0.1" > ./certs/san.ext
	openssl x509 -req -in ./certs/tls.csr -CA ./certs/ca.crt -CAkey ./certs/ca.key -CAcreateserial -out ./certs/tls.crt -days 365 -extfile ./certs/san.ext > /dev/null 2>&1

integration: kind load-kind install
	GOFLAGS="-gcflags=all=-lang=$(GOVERSION)" GOTOOLCHAIN=$(GOVERSION).0+auto go test -v -count=1 ./integration/... -vet=off -args --ginkgo.randomize-all && $(MAKE) delete-kind

.PHONY: run build image integration unit generate lint certs load-kind install kind delete-kind
