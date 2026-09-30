// internal/services/storage/processor_factory.go
package storage

import (
	"fmt"

	"github.com/combi/pkg/domain/queue"
	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
	"github.com/combi/storage/processors"
)

type processorFactory struct {
	logger            log.Logger
	imageProcessor    storage.FileProcessor
	videoProcessor    storage.FileProcessor
	documentProcessor storage.FileProcessor
}

// NewProcessorFactory now requires the queue dispatcher to pass to processors that need it.
func NewProcessorFactory(logger log.Logger, dispatcher queue.Dispatcher) storage.ProcessorFactory {
	return &processorFactory{
		logger:            logger.With(log.String("component", "processor_factory")),
		imageProcessor:    processors.NewImageProcessor(logger),
		videoProcessor:    processors.NewVideoProcessor(logger),
		documentProcessor: processors.NewDocumentProcessor(logger),
	}
}

// GetProcessor returns the correct file processor based on the task type string.
func (f *processorFactory) GetProcessor(taskType string) (storage.FileProcessor, error) {
	switch taskType {
	case "IMAGE_PROCESSOR":
		return f.imageProcessor, nil
	case "VIDEO_PROCESSOR":
		return f.videoProcessor, nil
	case "DOCUMENT_PROCESSOR":
		return f.documentProcessor, nil
	default:
		return nil, fmt.Errorf("%w: '%s'", storage.ErrUnsupportedProcessingTask, taskType)
	}
}
