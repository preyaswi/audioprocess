// pkg/api/handlers.go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"audio-processor/pkg/models"
	"audio-processor/pkg/pipeline"
	"audio-processor/pkg/storage"

	"github.com/gorilla/mux"
)

type Handlers struct {
	pipeline pipeline.Manager
	store    storage.MemoryStore
}

func NewHandlers(pipeline *pipeline.Manager, store storage.MemoryStore) *Handlers {
	return &Handlers{
		pipeline: *pipeline,
		store:    store,
	}
}

func (h *Handlers) UploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("user_id")
	sessionID := r.FormValue("session_id")

	if userID == "" || sessionID == "" {
		http.Error(w, "user_id and session_id are required", http.StatusBadRequest)
		return
	}

	var audioData []byte

	if file, _, err := r.FormFile("audio"); err == nil {
		defer file.Close()
		audioData, err = io.ReadAll(file)
		if err != nil {
			http.Error(w, "Failed to read audio file", http.StatusInternalServerError)
			return
		}
	}
	// else {
	// 	// Try reading raw body
	// 	audioData, err = io.ReadAll(r.Body)
	// 	if err != nil {
	// 		http.Error(w, "Failed to read audio data", http.StatusInternalServerError)
	// 		return
	// 	}
	// }

	chunk := models.NewAudioChunk(userID, sessionID, audioData)

	log.Printf("🚀 PROCESSING STARTED: ChunkID=%s, UserID=%s, Size=%d bytes",
		chunk.ID, chunk.UserID, chunk.Size)

	if err := h.pipeline.SubmitChunk(chunk); err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit chunk: %v", err), http.StatusServiceUnavailable)
		return
	}

	response := map[string]interface{}{
		"chunk_id": chunk.ID,
		"status":   "submitted",
		"size":     chunk.Size,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) GetChunkHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chunkID := vars["id"]

	chunk, err := h.store.GetChunk(chunkID)
	if err != nil {
		if err == storage.ErrChunkNotFound {
			log.Printf("CHUNK NOT READY: ChunkID=%s (still processing or doesn't exist)", chunkID)
			http.Error(w, "CHUNK NOT READY:(still processing or doesn't exist)", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("CHUNK RETRIEVED: ChunkID=%s, Status=%s", chunkID, chunk.Status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chunk)
}

func (h *Handlers) GetSessionChunksHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	chunks, err := h.store.GetSessionChunks(userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if len(chunks) > limit {
		chunks = chunks[:limit]
	}

	response := map[string]interface{}{
		"user_id": userID,
		"chunks":  chunks,
		"count":   len(chunks),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
