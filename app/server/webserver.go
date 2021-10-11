package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v3"
	"github.com/super-flat/raft-poc/app/fsm"
)

func GetWebServer(clusterID uint64, nodeHost *dragonboat.NodeHost) *gin.Engine {
	w := &WebServer{clusterID: clusterID, nodeHost: nodeHost}

	r := gin.Default()

	r.GET("/ping", w.handlePing)
	// DB methods
	r.GET("/db/:key", w.handleGet)
	r.POST("/db/:key/:value", w.handleSet)
	// Cluster methods
	r.POST("/cluster/addnode/:node_id/:node_addr", w.handleAddNode)

	return r
}

type WebServer struct {
	clusterID uint64
	nodeHost  *dragonboat.NodeHost
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
	res, err := w.nodeHost.SyncRead(ctx, w.clusterID, query)
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

	cs := w.nodeHost.GetNoOPSession(w.clusterID)
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
	response, err := w.nodeHost.RequestAddNode(w.clusterID, uint64(nodeID), nodeAddress, 0, 3*time.Second)

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
