package messaging

import (
	"fmt"
	"strings"

	"github.com/combi/pkg/domain/messaging"
)

// Allowed mime types for uploads in chat
var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"application/pdf": true,
}

// ValidateAttachment checks if the metadata for a file message is valid.
func ValidateAttachment(meta map[string]any) error {
	mime, ok := meta["mime_type"].(string)
	if !ok || mime == "" {
		return fmt.Errorf("missing mime_type in metadata")
	}

	if !allowedMimeTypes[mime] {
		return messaging.ErrInvalidFileType
	}

	if _, ok := meta["url"].(string); !ok {
		return fmt.Errorf("missing url in metadata")
	}

	return nil
}

// FormatSystemMessage helper for consistent system notifications
func FormatSystemMessage(action string, actorName string) string {
	switch action {
	case "join":
		return fmt.Sprintf("%s joined the channel.", actorName)
	case "leave":
		return fmt.Sprintf("%s left the channel.", actorName)
	case "rename":
		return fmt.Sprintf("Channel was renamed by %s.", actorName)
	default:
		return "System update."
	}
}

// ExtractMentions finds @username patterns in content
// Useful if you implement notifications later
func ExtractMentions(content string) []string {
	var mentions []string
	words := strings.Fields(content)
	for _, w := range words {
		if strings.HasPrefix(w, "@") && len(w) > 1 {
			mentions = append(mentions, w[1:])
		}
	}
	return mentions
}
