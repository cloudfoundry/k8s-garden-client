module code.cloudfoundry.org/k8s-garden-client

go 1.25.0

tool github.com/maxbrunsfeld/counterfeiter/v6

require (
	code.cloudfoundry.org/archiver v0.62.0
	code.cloudfoundry.org/bbs v0.0.0-20260107153229-7b22834eb1d7
	code.cloudfoundry.org/cacheddownloader v0.0.0-20250312193827-23c030d5e4f3
	code.cloudfoundry.org/clock v1.61.0
	code.cloudfoundry.org/commandrunner v0.57.0
	code.cloudfoundry.org/debugserver v0.86.0
	code.cloudfoundry.org/diego-logging-client v0.92.0
	code.cloudfoundry.org/durationjson v0.64.0
	code.cloudfoundry.org/executor v0.0.0-20260115133538-8eaf4cb3970e
	code.cloudfoundry.org/garden v0.0.0-20260211023825-fb202bdd1b27
	code.cloudfoundry.org/go-loggregator/v10 v10.2.0
	code.cloudfoundry.org/guardian v0.0.0-20260218024445-0b3043cea204
	code.cloudfoundry.org/lager/v3 v3.61.0
	code.cloudfoundry.org/localip v0.63.0
	code.cloudfoundry.org/locket v0.0.0-20251117222557-be612341b29d
	code.cloudfoundry.org/operationq v0.0.0-20250911185924-b1c6a9c2437f
	code.cloudfoundry.org/rep v0.0.0-20251217184847-b5d35dccad88
	code.cloudfoundry.org/tlsconfig v0.45.0
	code.cloudfoundry.org/volman v0.0.0-20250910193608-1cc72f1031b7
	code.cloudfoundry.org/workpool v0.0.0-20250911194158-1489753f182e
	github.com/containerd/containerd/api v1.10.0
	github.com/containerd/containerd/v2 v2.2.1
	github.com/containerd/continuity v0.4.5
	github.com/containerd/platforms v1.0.0-rc.2
	github.com/distribution/reference v0.6.0
	github.com/go-logr/logr v1.4.3
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.6.0
	github.com/jarcoal/httpmock v1.4.1
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo/v2 v2.28.1
	github.com/onsi/gomega v1.39.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/tedsuo/ifrit v0.0.0-20230516164442-7862c310ad26
	github.com/tedsuo/rata v1.0.0
	k8s.io/api v0.35.1
	k8s.io/apimachinery v0.35.1
	k8s.io/client-go v0.35.1
	k8s.io/kubelet v0.35.1
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2
	sigs.k8s.io/controller-runtime v0.23.1
)

require (
	cel.dev/expr v0.25.1 // indirect
	code.cloudfoundry.org/bytefmt v0.56.0 // indirect
	code.cloudfoundry.org/cfhttp/v2 v2.51.0 // indirect
	code.cloudfoundry.org/dockerdriver v0.62.0 // indirect
	code.cloudfoundry.org/ecrhelper v0.0.0-20250911193847-5bf65e63bab5 // indirect
	code.cloudfoundry.org/eventhub v0.56.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20260209061029-a81ffbc46978 // indirect
	code.cloudfoundry.org/go-loggregator/v9 v9.2.1 // indirect
	code.cloudfoundry.org/goshims v0.84.0 // indirect
	code.cloudfoundry.org/routing-info v0.0.0-20250117183711-d8d8d2ad4608 // indirect
	cyphar.com/go-pathrs v0.2.3 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.14.0-rc.1 // indirect
	github.com/aws/aws-sdk-go v1.55.8 // indirect
	github.com/aws/aws-sdk-go-v2 v1.39.6 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.31.17 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.21 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.51.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.38.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.39.1 // indirect
	github.com/aws/smithy-go v1.23.2 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.10.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudfoundry/dropsonde v1.1.0 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20251217143644-d1670a435f2a // indirect
	github.com/cncf/xds/go v0.0.0-20251210132809-ee656c7534f5 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/plugin v1.0.0 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.36.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-test/deep v1.1.1 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260202012954-cb029daf43ef // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jackc/pgx/v5 v5.7.6 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.12.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/reexec v0.1.0 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/cgroups v0.0.6 // indirect
	github.com/opencontainers/selinux v1.13.1 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/vito/go-sse v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.65.0 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/grpc v1.79.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.35.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2-0.20260122202528-d9cc6641c482 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
