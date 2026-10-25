package main

import (
	"encoding/json"
	"log"
	"time"
)

// logEvent prints one structured JSON log line.
// Never pass secrets, tokens, or full national IDs in fields.
func logEvent(level, msg string, fields map[string]any) {
	entry := map[string]any{
		"level":   level,
		"time":    time.Now().UTC().Format(time.RFC3339),
		"service": "rcn",
		"msg":     msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	b, err := json.Marshal(entry)
	if err != nil {
		log.Printf("%s: %s (log marshal error: %v)", level, msg, err)
		return
	}
	log.Println(string(b))
}
