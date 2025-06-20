package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"audio-processor/pkg/models"

	"github.com/joho/godotenv"
)

func (m *Manager) validateChunk(ctx context.Context, msg *models.PipelineMessage) {
	if len(msg.Chunk.Data) == 0 {
		msg.Error = fmt.Errorf("empty audio data")
		msg.Processed.Status = models.StatusFailed
		msg.Processed.Error = msg.Error.Error()
		return
	}

	if len(msg.Chunk.Data) > 10*1024*1024 {
		msg.Error = fmt.Errorf("audio chunk too large")
		msg.Processed.Status = models.StatusFailed
		msg.Processed.Error = msg.Error.Error()
		return
	}

	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

	msg.Processed.Status = models.StatusTransforming
	msg.Stage = "validation"

	select {
	case m.transformationCh <- msg:
	case <-ctx.Done():
		return
	}
}

func (m *Manager) transformChunk(ctx context.Context, msg *models.PipelineMessage) {
	hasher := sha256.New()
	hasher.Write(msg.Chunk.Data)
	msg.Processed.Checksum = fmt.Sprintf("%x", hasher.Sum(nil))

	msg.Processed.FFTData = m.mockFFT(msg.Chunk.Data)

	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

	msg.Processed.Status = models.StatusExtracting
	msg.Stage = "transformation"

	select {
	case m.extractionCh <- msg:
	case <-ctx.Done():
		return
	}
}

func (m *Manager) extractMetadata(ctx context.Context, msg *models.PipelineMessage) {
	msg.Processed.Transcript = m.mockTranscript(msg.Chunk.Data)

	msg.Processed.Metadata["duration"] = float64(len(msg.Chunk.Data)) / 44100.0 // Assume 44.1kHz
	msg.Processed.Metadata["sample_rate"] = 44100
	msg.Processed.Metadata["channels"] = 2

	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)

	msg.Processed.Status = models.StatusStoring
	msg.Stage = "extraction"

	select {
	case m.storageCh <- msg:
	case <-ctx.Done():
		return
	}
}

func (m *Manager) storeChunk(ctx context.Context, msg *models.PipelineMessage) {
	if err := m.memStore.StoreChunk(msg.Processed); err != nil {
		msg.Error = fmt.Errorf("failed to store in memory: %w", err)
		msg.Processed.Status = models.StatusFailed
		msg.Processed.Error = msg.Error.Error()
		return
	}

	if err := m.diskStore.StoreChunk(msg.Processed); err != nil {
		msg.Error = fmt.Errorf("failed to store on disk: %w", err)
		msg.Processed.Status = models.StatusFailed
		msg.Processed.Error = msg.Error.Error()
		return
	}

	msg.Processed.Status = models.StatusCompleted
	msg.Processed.ProcessedAt = time.Now()
	msg.Stage = "storage"

	m.memStore.UpdateChunkStatus(msg.Processed.ID, models.StatusCompleted)
}

func (m *Manager) mockFFT(data []byte) []float64 {
	size := min(len(data)/2, 1024)
	fft := make([]float64, size)

	for i := 0; i < size; i++ {
		fft[i] = rand.Float64() * 100
	}

	return fft
}

func (m *Manager) mockTranscript_old(data []byte) string {
	words := []string{"hello", "world", "audio", "processing", "system", "working", "perfectly"}

	wordCount := len(data) / 1000
	if wordCount > 10 {
		wordCount = 10
	}
	if wordCount < 1 {
		wordCount = 1
	}

	transcript := ""
	for i := 0; i < wordCount; i++ {
		if i > 0 {
			transcript += " "
		}
		transcript += words[rand.Intn(len(words))]
	}

	return transcript
}

func (m *Manager) mockTranscript(data []byte) string {
	transcript, err := m.transcribeWithWhisper(data)
	if err != nil {
		log.Printf("Transcription failed: %v, falling back to mock", err)
		return m.mockTranscript(data)
	}
	return transcript
}

func (m *Manager) transcribeWithWhisper(audioData []byte) (string, error) {

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	tempFile, err := os.CreateTemp("", "audio_*.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := tempFile.Write(audioData); err != nil {
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}
	tempFile.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	fileWriter, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	file, err := os.Open(tempFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(fileWriter, file); err != nil {
		return "", fmt.Errorf("failed to copy file data: %w", err)
	}

	writer.WriteField("model", "whisper-1")
	writer.WriteField("response_format", "text")
	writer.Close()

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	transcript, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(transcript), nil
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
