package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/super-flat/parti/logging"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsPath = "/metrics"
)

// Server is a wrapper around the started http.Server exposing the metrics
// The server needs to be created and ready to go before any metrics can be properly exported.
type Server struct {
	port       int
	httpServer *http.Server
}

// NewPromServer create an instance of PrometheusServer
func NewPromServer(port int) *Server {
	// Configure the metrics route
	mux := http.NewServeMux()
	// TODO we can add an error log handler later on
	mux.Handle(metricsPath, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	// Create the Server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// return the prometheus
	return &Server{
		port:       port,
		httpServer: server,
	}
}

// Start will start the prometheus server
func (p *Server) Start() {
	// Start Listening
	go func() {
		if err := p.httpServer.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				logging.Fatalf("failed to run Prometheus %s endpoint: %v", metricsPath, err)
			}
		}
	}()
	logging.Infof(" Prometheus %s endpoint started on %s ", metricsPath, p.httpServer.Addr)
}

// Stop stops the prometheus server
func (p Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := p.httpServer.Shutdown(ctx)
	if err != nil {
		logging.Errorf("failed to stop Prometheus %s endpoint: %v", metricsPath, err)
	}
}
