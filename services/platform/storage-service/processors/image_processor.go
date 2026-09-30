package processors

import (
	"context"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// imageProcessor is a stubbed-out version for environments where CGO/libvips is not available.
type imageProcessor struct {
	logger log.Logger
}

func NewImageProcessor(logger log.Logger) storage.FileProcessor {
	logger.Warn("Image processor is running in STUB mode (CGO/libvips disabled)")
	return &imageProcessor{
		logger: logger.With(log.String("processor", "image_stub")),
	}
}

func (p *imageProcessor) GetSupportedTasks() []string {
	return []string{"IMAGE_PROCESSOR"}
}

func (p *imageProcessor) Process(ctx context.Context, inputPath string, options map[string]interface{}) (*storage.ProcessingResult, error) {
	p.logger.Info("Image processing requested but skipped in stub mode")
	return &storage.ProcessingResult{
		Success: true,
		Data: map[string]interface{}{
			"variants": make(map[string]string),
			"stub":     true,
		},
	}, nil
}
