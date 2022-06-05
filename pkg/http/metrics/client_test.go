package metrics

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestGetClient(t *testing.T) {
	// create a prometheus registry
	r := prometheus.NewRegistry()
	// create a http client wrapper that can be used for
	// instrumentation
	clientWrapper := ClientWrapper{
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // nolint
				},
			},
		},
		Registerer: r,
	}
	// get an observable http client to call service a
	ca, err := clientWrapper.GetClient("troop.com")
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	// create a server
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	// replace the generated 127.0.0.1 with localhost domain that will enable us to collect dns metrics as well
	url := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	if _, err := ca.Get(url); err != nil {
		t.Fatal(err)
	}
	// assert the number of metrics type collected
	assertMetrics(t, r, 7)
}

// assertMetrics help count the number of metrics type that have been collected
func assertMetrics(t *testing.T, r prometheus.Gatherer, exp int) {
	var count int
	metricFamilies, err := r.Gather()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
	for _, mf := range metricFamilies {
		count += len(mf.Metric)
		t.Log(mf.GetName(), "-", mf.GetHelp())
	}
	if count != exp {
		t.Errorf("wrong number of metrics, expected %d got %d", exp, count)
	}
}
