package storage

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "audio-processor/pkg/models"
    "github.com/dgraph-io/badger/v3"
)

type DiskStore interface {
    StoreChunk(chunk *models.ProcessedChunk) error
    GetChunk(id string) (*models.ProcessedChunk, error)
    Close() error
}

type diskStore struct {
    db *badger.DB
}

func NewDiskStore(path string) (DiskStore, error) {
    if err := os.MkdirAll(path, 0755); err != nil {
        return nil, fmt.Errorf("failed to create storage directory: %w", err)
    }

    opts := badger.DefaultOptions(filepath.Join(path, "badger"))
    opts.Logger = nil // Disable badger logging
    
    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("failed to open badger database: %w", err)
    }

    return &diskStore{db: db}, nil
}

func (s *diskStore) StoreChunk(chunk *models.ProcessedChunk) error {
    data, err := json.Marshal(chunk)
    if err != nil {
        return fmt.Errorf("failed to marshal chunk: %w", err)
    }

    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set([]byte(chunk.ID), data)
    })
}

func (s *diskStore) GetChunk(id string) (*models.ProcessedChunk, error) {
    var chunk models.ProcessedChunk
    
    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte(id))
        if err != nil {
            return err
        }
        
        return item.Value(func(val []byte) error {
            return json.Unmarshal(val, &chunk)
        })
    })
    
    if err == badger.ErrKeyNotFound {
        return nil, ErrChunkNotFound
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to get chunk: %w", err)
    }
    
    return &chunk, nil
}

func (s *diskStore) Close() error {
    return s.db.Close()
}

var ErrChunkNotFound = fmt.Errorf("chunk not found")