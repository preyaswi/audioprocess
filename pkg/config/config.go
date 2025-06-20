package config

import (
    "time"
)

type Config struct {
    Server      ServerConfig
    Pipeline    PipelineConfig
    StoragePath string
}

type ServerConfig struct {
    Address      string
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
}

type PipelineConfig struct {
    ValidationWorkers     int
    TransformationWorkers int
    ExtractionWorkers     int
    StorageWorkers        int
    QueueSize            int
    ProcessingTimeout    time.Duration
}

func Load() *Config {
    return &Config{
        Server: ServerConfig{
            Address:      ":8080",
            ReadTimeout:  30 * time.Second,
            WriteTimeout: 30 * time.Second,
        },
        Pipeline: PipelineConfig{
            ValidationWorkers:     4,
            TransformationWorkers: 8,
            ExtractionWorkers:     4,
            StorageWorkers:        2,
            QueueSize:            1000,
            ProcessingTimeout:    5 * time.Minute,
        },
        StoragePath: "./data",
    }
}