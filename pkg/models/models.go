package models

import (
    "time"

    "github.com/google/uuid"
)

type AudioChunk struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    SessionID string    `json:"session_id"`
    Data      []byte    `json:"data"`
    Timestamp time.Time `json:"timestamp"`
    Size      int       `json:"size"`
}

type ProcessedChunk struct {
    ID           string            `json:"id"`
    UserID       string            `json:"user_id"`
    SessionID    string            `json:"session_id"`
    Timestamp    time.Time         `json:"timestamp"`
    Size         int               `json:"size"`
    Checksum     string            `json:"checksum"`
    FFTData      []float64         `json:"fft_data,omitempty"`
    Transcript   string            `json:"transcript,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
    ProcessedAt  time.Time         `json:"processed_at"`
    Status       ProcessingStatus  `json:"status"`
    Error        string            `json:"error,omitempty"`
}

type ProcessingStatus string

const (
    StatusPending     ProcessingStatus = "pending"
    StatusValidating  ProcessingStatus = "validating"
    StatusTransforming ProcessingStatus = "transforming"
    StatusExtracting  ProcessingStatus = "extracting"
    StatusStoring     ProcessingStatus = "storing"
    StatusCompleted   ProcessingStatus = "completed"
    StatusFailed      ProcessingStatus = "failed"
)

type PipelineMessage struct {
    Chunk   *AudioChunk
    Processed *ProcessedChunk
    Error   error
    Stage   string
}

func NewAudioChunk(userID, sessionID string, data []byte) *AudioChunk {
    return &AudioChunk{
        ID:        uuid.New().String(),
        UserID:    userID,
        SessionID: sessionID,
        Data:      data,
        Timestamp: time.Now(),
        Size:      len(data),
    }
}

func NewProcessedChunk(chunk *AudioChunk) *ProcessedChunk {
    return &ProcessedChunk{
        ID:        chunk.ID,
        UserID:    chunk.UserID,
        SessionID: chunk.SessionID,
        Timestamp: chunk.Timestamp,
        Size:      chunk.Size,
        Metadata:  make(map[string]interface{}),
        Status:    StatusPending,
    }
}