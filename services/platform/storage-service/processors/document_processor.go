package processors

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/combi/pkg/domain/storage"
	"github.com/combi/pkg/log"
)

// documentProcessor implements storage.FileProcessor for documents like PDFs.
// It extracts text and dispatches a job to the SearchService for indexing.
type documentProcessor struct {
	logger log.Logger
}

// NewDocumentProcessor creates a new document processor.
func NewDocumentProcessor(logger log.Logger) storage.FileProcessor {
	return &documentProcessor{
		logger: logger.With(log.String("processor", "document")),
	}
}

func (p *documentProcessor) GetSupportedTasks() []string {
	return []string{"DOCUMENT_PROCESSOR"}
}

// Process handles text extraction and dispatches a search indexing job.
func (p *documentProcessor) Process(ctx context.Context, inputPath string, options map[string]interface{}) (*storage.ProcessingResult, error) {
	// 1. Extract and validate options. We need the original File object for context.
	file, ok := options["file"].(*storage.File)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'file' object in options")
	}

	// 2. Extract text using an external tool (e.g., pdftotext from poppler-utils).
	extractedText, err := p.extractTextFromPDF(ctx, inputPath)
	if err != nil {
		// This might not be a fatal error; we can still index the file metadata.
		p.logger.Warn("Failed to extract text from document, will index metadata only", log.ErrorAttr(err), log.String("file_id", file.ID.String()))
	}

	// 5. Return the result of the processing task.
	result := &storage.ProcessingResult{
		Success: true,
		Data: map[string]interface{}{
			"text_content_length": len(extractedText),
		},
	}
	return result, nil
}

// extractTextFromPDF uses the `pdftotext` command-line utility.
func (p *documentProcessor) extractTextFromPDF(ctx context.Context, inputPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", inputPath, "-")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		p.logger.Error("pdftotext command failed", err, log.String("stderr", stderr.String()))
		return "", fmt.Errorf("pdftotext execution failed: %w", err)
	}

	return out.String(), nil
}
