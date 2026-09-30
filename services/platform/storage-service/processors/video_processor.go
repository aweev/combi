package processors

import (
	"context"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// videoProcessor is a stubbed-out version for environments where FFmpeg/system tools are not available or for simplified testing.
type videoProcessor struct {
	logger log.Logger
}

func NewVideoProcessor(logger log.Logger) storage.FileProcessor {
	logger.Warn("Video processor is running in STUB mode")
	return &videoProcessor{
		logger: logger.With(log.String("processor", "video_stub")),
	}
}

func (p *videoProcessor) GetSupportedTasks() []string {
	return []string{"VIDEO_PROCESSOR"}
}

func (p *videoProcessor) Process(ctx context.Context, inputPath string, options map[string]interface{}) (*storage.ProcessingResult, error) {
	p.logger.Info("Video processing requested but skipped in stub mode")
	return &storage.ProcessingResult{
		Success: true,
		Data: map[string]interface{}{
			"variants": make(map[string]string),
			"stub":     true,
		},
	}, nil
}
