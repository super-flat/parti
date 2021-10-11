package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lni/dragonboat/v3"
	"github.com/super-flat/raft-poc/app/fsm"
)

func GetWebServer(nodeHost *dragonboat.NodeHost) *gin.Engine {
	w := &WebServer{nodeHost: nodeHost}

	r := gin.Default()

	r.GET("/ping", w.handlePing)
	r.GET("/db/:key", w.handleGet)
	r.POST("/db/:key/:value", w.handleSet)

	return r
}

type WebServer struct {
	nodeHost *dragonboat.NodeHost
}

func (w *WebServer) handlePing(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func (w *WebServer) handleGet(c *gin.Context) {
	key := c.Param("key")
	fmt.Println("******* handle get")

	query := fsm.Query{Key: key}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	res, err := w.nodeHost.SyncRead(ctx, exampleClusterID, query)
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
	cs := w.nodeHost.GetNoOPSession(exampleClusterID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	result, err := w.nodeHost.SyncPropose(ctx, cs, []byte(value))
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
