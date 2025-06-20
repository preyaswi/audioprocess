package storage

import (
	"fmt"
	"sync"

	"audio-processor/pkg/models"
)

type MemoryStore interface {
	StoreChunk(chunk *models.ProcessedChunk) error
	GetChunk(id string) (*models.ProcessedChunk, error)
	GetSessionChunks(userID string) ([]*models.ProcessedChunk, error)
	UpdateChunkStatus(id string, status models.ProcessingStatus) error
}

type memoryStore struct {
	chunks map[string]*models.ProcessedChunk
	mu     sync.RWMutex
}

func NewMemoryStore() MemoryStore {
	return &memoryStore{
		chunks: make(map[string]*models.ProcessedChunk),
	}
}

func (s *memoryStore) StoreChunk(chunk *models.ProcessedChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Println("the chunk is storing")
	s.chunks[chunk.ID] = chunk
	fmt.Println("the chunk is stored")
	return nil
}

func (s *memoryStore) GetChunk(id string) (*models.ProcessedChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chunk, exists := s.chunks[id]
	if !exists {
		return nil, ErrChunkNotFound
	}

	return chunk, nil
}

func (s *memoryStore) GetSessionChunks(userID string) ([]*models.ProcessedChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chunks []*models.ProcessedChunk
	for _, chunk := range s.chunks {
		if chunk.UserID == userID {
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

func (s *memoryStore) UpdateChunkStatus(id string, status models.ProcessingStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	chunk, exists := s.chunks[id]
	if !exists {
		return ErrChunkNotFound
	}

	chunk.Status = status
	return nil
}
