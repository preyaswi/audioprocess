package pipeline

import (
    "context"
    "sync"

    "audio-processor/pkg/models"
)

type WorkerPool struct {
    workers    int
    taskQueue  chan *models.PipelineMessage
    workerFunc func(context.Context, *models.PipelineMessage)
    wg         sync.WaitGroup
}

func NewWorkerPool(workers int, workerFunc func(context.Context, *models.PipelineMessage)) *WorkerPool {
    return &WorkerPool{
        workers:    workers,
        taskQueue:  make(chan *models.PipelineMessage, workers*2),
        workerFunc: workerFunc,
    }
}

func (wp *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < wp.workers; i++ {
        wp.wg.Add(1)
        go wp.worker(ctx)
    }
}

func (wp *WorkerPool) Submit(msg *models.PipelineMessage) {
    wp.taskQueue <- msg
}

func (wp *WorkerPool) Stop() {
    close(wp.taskQueue)
    wp.wg.Wait()
}

func (wp *WorkerPool) worker(ctx context.Context) {
    defer wp.wg.Done()
    
    for {
        select {
        case msg, ok := <-wp.taskQueue:
            if !ok {
                return
            }
            wp.workerFunc(ctx, msg)
            
        case <-ctx.Done():
            return
        }
    }
}