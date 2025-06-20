// pkg/api/websocket.go
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"audio-processor/pkg/models"
	"audio-processor/pkg/storage"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketMessage struct {
	Type      string          `json:"type"`
	UserID    string          `json:"user_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	ChunkID   string          `json:"chunk_id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func (h *Handlers) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		var msg WebSocketMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "audio_chunk":
			h.handleAudioChunk(ctx, conn, &msg)
		case "ping":
			h.sendMessage(conn, WebSocketMessage{
				Type: "pong",
			})
		default:
			h.sendMessage(conn, WebSocketMessage{
				Type:  "error",
				Error: "Unknown message type",
			})
		}
	}
}

func (h *Handlers) handleAudioChunk(ctx context.Context, conn *websocket.Conn, msg *WebSocketMessage) {
	if msg.UserID == "" || msg.SessionID == "" {
		h.sendMessage(conn, WebSocketMessage{
			Type:  "error",
			Error: "user_id and session_id are required",
		})
		return
	}

	var audioData []byte
	if err := json.Unmarshal(msg.Data, &audioData); err != nil {
		h.sendMessage(conn, WebSocketMessage{
			Type:  "error",
			Error: "Invalid audio data format",
		})
		return
	}

	chunk := models.NewAudioChunk(msg.UserID, msg.SessionID, audioData)

	log.Printf("🚀 WS PROCESSING STARTED: ChunkID=%s, UserID=%s, Size=%d bytes", 
		chunk.ID, chunk.UserID, chunk.Size)

	h.sendMessage(conn, WebSocketMessage{
		Type:    "chunk_received",
		ChunkID: chunk.ID,
		Status:  "submitted",
	})

	if err := h.pipeline.SubmitChunk(chunk); err != nil {
		h.sendMessage(conn, WebSocketMessage{
			Type:    "error",
			ChunkID: chunk.ID,
			Error:   err.Error(),
		})
		return
	}

	go h.monitorChunkProcessing(ctx, conn, chunk.ID)
}

func (h *Handlers) monitorChunkProcessing(ctx context.Context, conn *websocket.Conn, chunkID string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			chunk, err := h.store.GetChunk(chunkID)
			if err != nil {
				if err != storage.ErrChunkNotFound {
					h.sendMessage(conn, WebSocketMessage{
						Type:    "error",
						ChunkID: chunkID,
						Error:   err.Error(),
					})
				}
				return
			}

			h.sendMessage(conn, WebSocketMessage{
				Type:    "status_update",
				ChunkID: chunkID,
				Status:  string(chunk.Status),
			})

			if chunk.Status == models.StatusCompleted {
				log.Printf("WS PROCESSING COMPLETED: ChunkID=%s, Status=%s", chunkID, chunk.Status)
				log.Printf("CHUNK READY FOR GET: ChunkID=%s available at GET /chunks/%s", chunkID, chunkID)
				
				h.sendMessage(conn, WebSocketMessage{
					Type:    "processing_complete",
					ChunkID: chunkID,
					Data:    mustMarshal(chunk),
				})
				return
			}

			if chunk.Status == models.StatusFailed {
				log.Printf("WS PROCESSING FAILED: ChunkID=%s, Error=%s", chunkID, chunk.Error)
				
				h.sendMessage(conn, WebSocketMessage{
					Type:    "processing_failed",
					ChunkID: chunkID,
					Error:   chunk.Error,
				})
				return
			}
		}
	}
}

func (h *Handlers) sendMessage(conn *websocket.Conn, msg WebSocketMessage) {
	if err := conn.WriteJSON(msg); err != nil {
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}