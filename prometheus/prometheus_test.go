package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PrometheusServerSuite struct {
	suite.Suite
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestPrometheusServerSuite(t *testing.T) {
	suite.Run(t, new(PrometheusServerSuite))
}

func (s *PrometheusServerSuite) TestStart() {
	const port = 8888
	server := NewPromServer(port)
	s.Assert().NotNil(server)
	s.Assert().NotNil(server.httpServer)
	s.Assert().Equal(fmt.Sprintf(":%d", port), server.httpServer.Addr)
	s.Require().IsType(http.DefaultServeMux, server.httpServer.Handler)

	// let us start the server
	server.Start()
	mux := server.httpServer.Handler.(*http.ServeMux)
	handler, pattern := mux.Handler(&http.Request{
		URL: &url.URL{
			Host: "localhost",
			Path: "/metrics",
		},
	})
	s.Assert().Equal("/metrics", pattern)
	s.Assert().NotNil(handler)

	err := server.httpServer.Shutdown(context.Background())
	s.Assert().NoError(err)
}

func (s *PrometheusServerSuite) TestStop() {
	// Start a local HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write([]byte(`OK`))
	}))

	prometheusSrv := &Server{
		httpServer: httpServer.Config,
	}

	resp, err := http.Get(httpServer.URL)
	s.Require().NoError(err)
	s.Assert().Equal(resp.StatusCode, 200)

	prometheusSrv.Stop()

	_, err = http.Get(httpServer.URL)
	s.Require().Error(err)
}
