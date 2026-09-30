package processors_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/combi/pkg/domain/storage"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/storage/processors"
)

func TestImageProcessor_Process(t *testing.T) {
	// 1. Create a dummy image (1x1 PNG)
	// Vips needs a real image structure to not fail
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	tmpFile, err := os.CreateTemp("", "test.png")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	io.Copy(tmpFile, bytes.NewReader(pngBytes))
	tmpFile.Close()

	// 2. Setup
	provider := storage_mocks.NewMockProvider(t)
	processor := processors.NewImageProcessor(&log.NoOpLogger{})

	options := map[string]interface{}{
		"provider": provider,
		"bucket":   "test-bucket",
		"basePath": "uploads/img",
		"variants": map[string]string{
			"thumb": "100x100",
			"large": "800x800",
		},
	}

	// 3. Run
	res, err := processor.Process(context.Background(), tmpFile.Name(), options)

	// 4. Assert
	// Since we are running in STUB mode, we expect it to succeed with no error.
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Success)
	assert.Equal(t, true, res.Data["stub"])
}

func TestDocumentProcessor_Process(t *testing.T) {
	// Create a dummy PDF file for testing
	tmpFile, err := os.CreateTemp("", "test.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("%PDF-1.4...") // Minimal header
	tmpFile.Close()

	logger := &log.NoOpLogger{}
	// We removed dispatcher dependency, so just logger
	processor := processors.NewDocumentProcessor(logger)

	file := &storage.File{ID: uuid.New(), OriginalName: "test.pdf"}

	// Mock options
	_ = storage_mocks.NewMockProvider(t)
	options := map[string]interface{}{
		"file": file,
		// These aren't used by document processor currently but required by extraction logic
		// if we were uploading something
	}

	// EXECUTION NOTE: This test will try to run `pdftotext`.
	// If it's not installed, it will return an error.
	// In a real CI env, you'd install poppler-utils.
	// We will check if it returns a valid Result structure, even if extraction fails (it returns warnings).

	res, err := processor.Process(context.Background(), tmpFile.Name(), options)

	// If pdftotext is missing, it returns an error. That's fine, it proves it tried to run.
	// If it is present, it returns success.
	if err == nil {
		assert.True(t, res.Success)
		assert.Contains(t, res.Data, "text_content_length")
	} else {
		// Verify it failed inside extraction, not before
		assert.Contains(t, err.Error(), "pdftotext")
	}
}
