package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v3"
	"github.com/super-flat/raft-poc/app/fsm"
)

func NewWebServer(clusterID uint64, nodeID uint64, raftPort string, apiPort string) *WebServer {
	w := &WebServer{
		raftClusterID: clusterID,
		raftNodeID:    nodeID,
		raftPort:      raftPort,
		raftJoinable:  false,
		apiPort:       apiPort,
	}
	r := gin.Default()
	w.router = r

	r.GET("/ping", w.handlePing)
	// DB methods
	r.GET("/db/:key", w.handleGet)
	r.POST("/db/:key/:value", w.handleSet)
	// Cluster methods
	r.POST("/cluster/addnode/:node_id/:node_addr", w.handleAddNode)
	// tmp
	r.GET("/info", w.handleBootstrap)

	return w
}

type WebServer struct {
	raftClusterID uint64
	raftNodeID    uint64
	raftPort      string
	raftJoinable  bool

	apiPort  string
	router   *gin.Engine
	nodeHost *dragonboat.NodeHost
	srv      *http.Server
}

func (w *WebServer) isJoinable() bool {
	if w.nodeHost == nil {
		return false
	}
	_, hasLeader, err := w.nodeHost.GetLeaderID(w.raftClusterID)
	return err == nil && hasLeader
}

func (w *WebServer) Run() {
	w.srv = &http.Server{
		Addr:    fmt.Sprintf(":%s", w.apiPort),
		Handler: w.router,
	}

	go func() {
		// service connections
		if err := w.srv.ListenAndServe(); err != nil {
			log.Printf("listen: %s\n", err)
		}
	}()
}

func (w *WebServer) Stop(ctx context.Context) {
	if w.srv != nil {
		if err := w.srv.Shutdown(ctx); err != nil {
			log.Fatal("Server Shutdown:", err)
		}
		log.Println("Server exiting")
	}
}

func (w *WebServer) SetNodeHost(nodeHost *dragonboat.NodeHost) {
	w.nodeHost = nodeHost
}

func (w *WebServer) handlePing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func (w *WebServer) handleGet(c *gin.Context) {
	key := c.Param("key")

	query := fsm.Query{Key: key}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	res, err := w.nodeHost.SyncRead(ctx, w.raftClusterID, query)
	cancel()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"key":   key,
			"error": err.Error(),
		})
	} else {
		entry := res.(fsm.Entry)
		c.JSON(200, gin.H{
			"key":   key,
			"value": entry.Value,
		})
	}
}

func (w *WebServer) handleSet(c *gin.Context) {
	key := c.Param("key")
	value := c.Param("value")

	entry := &fsm.Entry{Key: key, Value: value}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"key":   key,
			"error": err.Error(),
		})
	}

	cs := w.nodeHost.GetNoOPSession(w.raftClusterID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	result, err := w.nodeHost.SyncPropose(ctx, cs, entryBytes)
	cancel()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"key":   key,
			"error": err.Error(),
		})
	} else {
		newValue := string(result.Data)
		c.JSON(200, gin.H{
			"key":   key,
			"value": newValue,
		})
	}

}

func (w *WebServer) handleAddNode(c *gin.Context) {
	nodeID, _ := strconv.Atoi(c.Param("node_id"))
	nodeAddress := c.Param("node_addr")
	response, err := w.nodeHost.RequestAddNode(w.raftClusterID, uint64(nodeID), nodeAddress, 0, 3*time.Second)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"nodeID":      nodeID,
			"nodeAddress": nodeAddress,
			"error":       err.Error(),
		})
	} else {
		outcome := <-response.AppliedC()

		if outcome.Completed() {
			c.JSON(200, gin.H{
				"nodeID":      nodeID,
				"nodeAddress": nodeAddress,
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"nodeID":      nodeID,
				"nodeAddress": nodeAddress,
			})
		}
	}
}

type BootstrapInfo struct {
	RaftClusterID uint64 `json:"raft_cluster_id"`
	RaftNodeID    uint64 `json:"raft_node_id"`
	RaftPort      string `json:"raft_port"`
	Joinable      bool   `json:"joinable"`
}

func (w *WebServer) handleBootstrap(c *gin.Context) {
	output := &BootstrapInfo{
		RaftClusterID: w.raftClusterID,
		RaftNodeID:    w.raftNodeID,
		RaftPort:      w.raftPort,
		Joinable:      w.isJoinable(),
	}
	c.JSON(http.StatusOK, output)
}
