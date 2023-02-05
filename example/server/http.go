package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/super-flat/parti/cluster"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// WebServer is an http server that forwards messages to the raft cluster
type WebServer struct {
	Parti    *cluster.Cluster
	HTTPPort uint16
	server   *http.Server
}

// NewWebServer returns a new WebServer
func NewWebServer(partiNode *cluster.Cluster, HTTPPort uint16) *WebServer {
	return &WebServer{
		Parti:    partiNode,
		HTTPPort: HTTPPort,
	}
}

// Start the webserver in background
func (wb *WebServer) Start() {
	// define a router that handles messages
	mux := http.NewServeMux()
	mux.HandleFunc("/send", wb.handleMessage)
	// create a webserver
	wb.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", wb.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: 60 * time.Second,
	}
	// run the server in a thread
	go func() {
		if err := wb.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Printf("webserver listening on '%s'", wb.server.Addr)
}

// Stop gracefully shuts down this webserver
func (wb *WebServer) Stop(ctx context.Context) {
	newCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := wb.server.Shutdown(newCtx); err != nil {
		log.Fatalf("webserver shutdown failed:%+v", err)
	}
	log.Print("example webserver shut down properly")
}

func (wb *WebServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	// read the partition
	partitionID, ok := parsePartition(r.URL.Query().Get("partition"))
	if !ok {
		log.Print("missing partition")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing partition"))
		return
	}
	// read the message
	msg, _ := anypb.New(wrapperspb.String(r.URL.Query().Get("message")))
	// make a request
	sendRequest := &partipb.SendRequest{
		PartitionId: partitionID,
		MessageId:   uuid.New().String(),
		Message:     msg,
	}
	resp, err := wb.Parti.Send(r.Context(), sendRequest)
	if err != nil {
		log.Print(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	format := &protojson.MarshalOptions{
		EmitUnpopulated: true,
		Multiline:       true,
	}
	_, _ = fmt.Fprint(w, format.Format(resp))
}

func parsePartition(value string) (uint32, bool) {
	if typedVal, err := strconv.ParseUint(value, 10, 32); err == nil {
		return uint32(typedVal), true
	}
	return 0, false
}
