package log_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"code.cloudfoundry.org/lager/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pkglog "code.cloudfoundry.org/k8s-garden-client/pkg/k8sgarden/log"
)

// Helpers to parse the last emitted JSON log and extract common fields.
func lastLog(buf *bytes.Buffer) map[string]any {
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	Expect(lines).NotTo(BeEmpty())
	var m map[string]any
	Expect(json.Unmarshal([]byte(lines[len(lines)-1]), &m)).To(Succeed())
	return m
}

func message(m map[string]any) string {
	msg, _ := m["message"].(string)
	return msg
}

func logData(m map[string]any) map[string]any {
	data, ok := m["data"].(map[string]any)
	Expect(ok).To(BeTrue())
	return data
}

var _ = Describe("KlogSink", func() {
	var (
		logger   lager.Logger
		sinkImpl logr.LogSink
		buf      *bytes.Buffer
	)

	BeforeEach(func() {
		logger = lager.NewLogger("klog-sink-test")
		buf = &bytes.Buffer{}
		logger.RegisterSink(lager.NewWriterSink(buf, lager.DEBUG))
		sinkImpl = pkglog.NewSink(logger)
	})

	It("reports enabled for any level", func() {
		Expect(sinkImpl.Enabled(0)).To(BeTrue())
		Expect(sinkImpl.Enabled(1)).To(BeTrue())
		Expect(sinkImpl.Enabled(5)).To(BeTrue())
		Expect(sinkImpl.Enabled(-1)).To(BeTrue())
	})

	It("logs Info with mapped key/value data", func() {
		sinkImpl.Info(0, "hello", "a", 1, "b", "x", 123, "ignored", "c", nil)

		m := lastLog(buf)
		Expect(message(m)).To(HaveSuffix(".hello"))
		data := logData(m)
		Expect(data["a"]).To(Equal(float64(1)))
		Expect(data["b"]).To(Equal("x"))
		Expect(data).NotTo(HaveKey("123"))
		// nil values serialize as null
		Expect(data["c"]).To(BeNil())
	})

	It("logs Error with mapped key/value data and error", func() {
		err := errors.New("boom")
		sinkImpl.Error(err, "oops", "k", "v")

		m := lastLog(buf)
		Expect(message(m)).To(HaveSuffix(".oops"))
		data := logData(m)
		Expect(data["k"]).To(Equal("v"))
	})

	It("supports Init without side effects", func() {
		// Should not panic or modify behavior
		sinkImpl.Init(logr.RuntimeInfo{})
		sinkImpl.Info(0, "post-init")
		m := lastLog(buf)
		Expect(message(m)).To(HaveSuffix(".post-init"))
	})

	It("creates a child sink with WithName that logs", func() {
		child := sinkImpl.WithName("child")
		child.Info(0, "child-msg")
		m := lastLog(buf)
		Expect(message(m)).To(HaveSuffix(".child-msg"))
	})

	It("attaches session values via WithValues", func() {
		withVals := sinkImpl.WithValues("origin", "alpha", "count", 2)
		withVals.Info(0, "session-msg")

		m := lastLog(buf)
		Expect(message(m)).To(HaveSuffix(".session-msg"))
		data := logData(m)
		Expect(data["origin"]).To(Equal("alpha"))
		Expect(data["count"]).To(Equal(float64(2)))
	})
})
