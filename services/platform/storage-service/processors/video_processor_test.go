package processors_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	storage_mocks "github.com/combi/pkg/domain/storage/mocks"
	"github.com/combi/pkg/log"
	"github.com/combi/storage/processors"
)

func TestVideoProcessor_Process_Flow(t *testing.T) {
	// We can't easily fake a video file that ffprobe accepts without a binary.
	// So we create a dummy file.
	tmpFile, _ := os.CreateTemp("", "test.mp4")
	defer os.Remove(tmpFile.Name())

	provider := storage_mocks.NewMockProvider(t)
	processor := processors.NewVideoProcessor(&log.NoOpLogger{})

	options := map[string]interface{}{
		"provider": provider,
		"bucket":   "test-bucket",
		"basePath": "video/vid",
	}

	// Run
	res, err := processor.Process(context.Background(), tmpFile.Name(), options)

	// Since we are running in STUB mode, we expect it to succeed with no error.
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Success)
	assert.Equal(t, true, res.Data["stub"])
}
