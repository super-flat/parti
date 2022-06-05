package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	httpOutgoing           = "http_outgoing"
	requestsTotal          = "requests_total"
	requestDurationSeconds = "request_duration_seconds"
	dnsDurationSeconds     = "dns_duration_seconds"
	tlsDurationSeconds     = "tls_duration_seconds"
	inflightRequests       = "in_flight_requests"
)

// ClientWrapper wraps the original http Client.
// It allows to create its copy that will be instrumented
type ClientWrapper struct {
	*http.Client
	Registerer prometheus.Registerer
	Namespace  string
}

// GetClient allocates new client based on base one with incomingInstrumentation.
// Given serviceName is used as a constant label.
func (c *ClientWrapper) GetClient(serviceName string) (*http.Client, error) {
	return instrumentClientWithConstLabels(c.Namespace, c.Client, c.Registerer, map[string]string{
		"service": serviceName,
	})
}

// instrumentClientWithConstLabels helps instruments metrics for the given http client
func instrumentClientWithConstLabels(namespace string, c *http.Client, reg prometheus.Registerer, constLabels map[string]string) (*http.Client, error) {
	// create an outgoing instrumentation
	oi := &outgoingInstrumentation{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Subsystem:   httpOutgoing,
				Name:        requestsTotal,
				Help:        "A counter for outgoing requests from the wrapped client.",
				ConstLabels: constLabels,
			},
			[]string{"code", "method"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   httpOutgoing,
				Name:        requestDurationSeconds,
				Help:        "A histogram of outgoing request latencies.",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: constLabels,
			},
			[]string{"method"},
		),
		dnsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   httpOutgoing,
				Name:        dnsDurationSeconds,
				Help:        "Trace dns latency histogram.",
				Buckets:     []float64{.005, .01, .025, .05},
				ConstLabels: constLabels,
			},
			[]string{"event"},
		),
		tlsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace:   namespace,
				Subsystem:   httpOutgoing,
				Name:        tlsDurationSeconds,
				Help:        "Trace tls latency histogram.",
				Buckets:     []float64{.05, .1, .25, .5},
				ConstLabels: constLabels,
			},
			[]string{"event"},
		),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   httpOutgoing,
			Name:        inflightRequests,
			Help:        "A gauge of in-flight outgoing requests for the wrapped client.",
			ConstLabels: constLabels,
		}),
	}

	trace := &promhttp.InstrumentTrace{
		DNSStart: func(t float64) {
			oi.dnsDuration.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			oi.dnsDuration.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			oi.tlsDuration.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			oi.tlsDuration.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}

	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		CheckRedirect: c.CheckRedirect,
		Jar:           c.Jar,
		Timeout:       c.Timeout,
		Transport: promhttp.InstrumentRoundTripperInFlight(oi.inflight,
			promhttp.InstrumentRoundTripperCounter(oi.requests,
				promhttp.InstrumentRoundTripperTrace(trace,
					promhttp.InstrumentRoundTripperDuration(oi.duration, transport),
				),
			),
		),
	}, reg.Register(oi)
}

// outgoingInstrumentation instrumentation
type outgoingInstrumentation struct {
	duration    *prometheus.HistogramVec
	requests    *prometheus.CounterVec
	dnsDuration *prometheus.HistogramVec
	tlsDuration *prometheus.HistogramVec
	inflight    prometheus.Gauge
}

// Describe implements prometheus.Collector interface.
func (i *outgoingInstrumentation) Describe(in chan<- *prometheus.Desc) {
	i.duration.Describe(in)
	i.requests.Describe(in)
	i.dnsDuration.Describe(in)
	i.tlsDuration.Describe(in)
	i.inflight.Describe(in)
}

// Collect implements prometheus.Collector interface.
func (i *outgoingInstrumentation) Collect(in chan<- prometheus.Metric) {
	i.duration.Collect(in)
	i.requests.Collect(in)
	i.dnsDuration.Collect(in)
	i.tlsDuration.Collect(in)
	i.inflight.Collect(in)
}
