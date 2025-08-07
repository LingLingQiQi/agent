package utils

import (
	"fmt"
	"net/http"
)

type SSEWriter struct {
	w http.ResponseWriter
}

func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	
	return &SSEWriter{w: w}
}

func (s *SSEWriter) Write(event, data string) error {
	if event != "" {
		if _, err := fmt.Fprintf(s.w, "event: %s\n", event); err != nil {
			return err
		}
	}
	
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
	
	return nil
}

func (s *SSEWriter) Close() error {
	return s.Write("", "[DONE]")
}