package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/super-flat/parti/gen/localpb"
	"github.com/super-flat/parti/node"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// WebServer is an http server that forwards messages to the raft cluster
type WebServer struct {
	Parti    *node.Node
	HttpPort uint16
	server   *http.Server
}

// NewWebServer returns a new WebServer
func NewWebServer(partiNode *node.Node, HttpPort uint16) *WebServer {
	return &WebServer{
		Parti:    partiNode,
		HttpPort: HttpPort,
	}
}

// Start the webserver in background
func (w *WebServer) Start() {
	// define a router that handles messages
	mux := http.NewServeMux()
	mux.HandleFunc("/send", w.handleMessage)
	// create a webserver
	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.HttpPort),
		Handler: mux,
	}
	// run the server in a thread
	go func() {
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Printf("webserver listening on '%s'", w.server.Addr)
}

// Stop gracefully shuts down this webserver
func (w *WebServer) Stop(ctx context.Context) {
	newCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := w.server.Shutdown(newCtx); err != nil {
		log.Fatalf("webserver shutdown failed:%+v", err)
	}
	log.Print("webserver shut down properly")
}

func (wb *WebServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	// read the partition
	partitionID, ok := parsePartition(r.URL.Query().Get("partition"))
	if !ok {
		log.Print("missing partition")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing partition"))
		return
	}
	// read the message
	msg, _ := anypb.New(wrapperspb.String(r.URL.Query().Get("message")))
	// make a request
	sendRequest := &localpb.SendRequest{
		PartitionId: partitionID,
		MessageId:   uuid.New().String(),
		Message:     msg,
	}
	resp, err := wb.Parti.Send(r.Context(), sendRequest)
	if err != nil {
		log.Print(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	format := &protojson.MarshalOptions{
		EmitUnpopulated: true,
		Multiline:       true,
	}
	fmt.Fprint(w, format.Format(resp))
}

func parsePartition(value string) (uint32, bool) {
	if typedVal, err := strconv.ParseUint(value, 10, 32); err == nil {
		return uint32(typedVal), true
	}
	return 0, false
}