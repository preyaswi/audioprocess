package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

	"audio-processor/pkg/config"
	"audio-processor/pkg/models"
	"audio-processor/pkg/storage"
)

type Manager struct {
	config    config.PipelineConfig
	memStore  storage.MemoryStore
	diskStore storage.DiskStore

	// Pipeline channels
	ingestionCh      chan *models.AudioChunk
	validationCh     chan *models.PipelineMessage
	transformationCh chan *models.PipelineMessage
	extractionCh     chan *models.PipelineMessage
	storageCh        chan *models.PipelineMessage

	// Worker pools
	validationPool     *WorkerPool
	transformationPool *WorkerPool
	extractionPool     *WorkerPool
	storagePool        *WorkerPool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewManager(cfg config.PipelineConfig, memStore storage.MemoryStore, diskStore storage.DiskStore) *Manager {
	return &Manager{
		config:    cfg,
		memStore:  memStore,
		diskStore: diskStore,

		ingestionCh:      make(chan *models.AudioChunk, cfg.QueueSize),
		validationCh:     make(chan *models.PipelineMessage, cfg.QueueSize),
		transformationCh: make(chan *models.PipelineMessage, cfg.QueueSize),
		extractionCh:     make(chan *models.PipelineMessage, cfg.QueueSize),
		storageCh:        make(chan *models.PipelineMessage, cfg.QueueSize),
	}
}

func (m *Manager) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)
	log.Println("Pipeline Manager: Starting...")

	// Initialize worker pools
	m.validationPool = NewWorkerPool(m.config.ValidationWorkers, m.validateChunk)
	m.transformationPool = NewWorkerPool(m.config.TransformationWorkers, m.transformChunk)
	m.extractionPool = NewWorkerPool(m.config.ExtractionWorkers, m.extractMetadata)
	m.storagePool = NewWorkerPool(m.config.StorageWorkers, m.storeChunk)

	// Start worker pools
	log.Println("Pipeline Manager: Starting worker pools...")
	m.validationPool.Start(m.ctx)
	m.transformationPool.Start(m.ctx)
	m.extractionPool.Start(m.ctx)
	m.storagePool.Start(m.ctx)

	// Start pipeline stages
	log.Println("Pipeline Manager: Starting pipeline stages...")
	m.wg.Add(5)
	go m.runIngestionStage()
	go m.runValidationStage()
	go m.runTransformationStage()
	go m.runExtractionStage()
	go m.runStorageStage()

	return nil
}

func (m *Manager) Stop() {
	log.Println("Pipeline Manager: Stopping...")
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	log.Println("Pipeline Manager: Stopped.")
}

func (m *Manager) SubmitChunk(chunk *models.AudioChunk) error {
	log.Printf("Pipeline Manager: Submitting chunk %s to ingestion channel.", chunk.ID)
	select {
	case m.ingestionCh <- chunk:
		log.Printf("Pipeline Manager: Chunk %s successfully submitted.", chunk.ID)
		return nil
	case <-m.ctx.Done():
		log.Printf("Pipeline Manager: Failed to submit chunk %s, pipeline is shutting down.", chunk.ID)
		return fmt.Errorf("pipeline is shutting down")
	default:
		log.Printf("Pipeline Manager: Failed to submit chunk %s, pipeline queue is full.", chunk.ID)
		return fmt.Errorf("pipeline queue is full")
	}
}

func (m *Manager) runIngestionStage() {
	defer m.wg.Done()
	log.Println("Ingestion Stage: Running.")

	for {
		select {
		case chunk := <-m.ingestionCh:
			log.Printf("Ingestion Stage: Received chunk %s.", chunk.ID)
			processed := models.NewProcessedChunk(chunk)
			processed.Status = models.StatusValidating

			msg := &models.PipelineMessage{
				Chunk:     chunk,
				Processed: processed,
				Stage:     "ingestion",
			}

			select {
			case m.validationCh <- msg:
				log.Printf("Ingestion Stage: Chunk %s sent to validation.", chunk.ID)
			case <-m.ctx.Done():
				log.Printf("Ingestion Stage: Shutting down, failed to send chunk %s to validation.", chunk.ID)
				return
			}

		case <-m.ctx.Done():
			log.Println("Ingestion Stage: Shutting down.")
			return
		}
	}
}

func (m *Manager) runValidationStage() {
	defer m.wg.Done()
	log.Println("Validation Stage: Running.")

	for {
		select {
		case msg := <-m.validationCh:
			log.Printf("Validation Stage: Received chunk %s for validation.", msg.Chunk.ID)
			m.validationPool.Submit(msg)

		case <-m.ctx.Done():
			log.Println("Validation Stage: Shutting down.")
			return
		}
	}
}

func (m *Manager) runTransformationStage() {
	defer m.wg.Done()
	log.Println("Transformation Stage: Running.")

	for {
		select {
		case msg := <-m.transformationCh:
			log.Printf("Transformation Stage: Received chunk %s for transformation.", msg.Chunk.ID)
			m.transformationPool.Submit(msg)

		case <-m.ctx.Done():
			log.Println("Transformation Stage: Shutting down.")
			return
		}
	}
}

func (m *Manager) runExtractionStage() {
	defer m.wg.Done()
	log.Println("Extraction Stage: Running.")

	for {
		select {
		case msg := <-m.extractionCh:
			log.Printf("Extraction Stage: Received chunk %s for extraction.", msg.Chunk.ID)
			m.extractionPool.Submit(msg)

		case <-m.ctx.Done():
			log.Println("Extraction Stage: Shutting down.")
			return
		}
	}
}

func (m *Manager) runStorageStage() {
	defer m.wg.Done()
	log.Println("Storage Stage: Running.")

	for {
		select {
		case msg := <-m.storageCh:
			log.Printf("Storage Stage: Received chunk %s for storage.", msg.Processed.ID)
			m.storagePool.Submit(msg)

		case <-m.ctx.Done():
			log.Println("Storage Stage: Shutting down.")
			return
		}
	}
}