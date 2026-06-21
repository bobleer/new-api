package common

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

// SessionTraceResponseWriter tees response bytes into a bounded buffer for session tracing.
type SessionTraceResponseWriter struct {
	gin.ResponseWriter
	buf      bytes.Buffer
	maxBytes int
}

func NewSessionTraceResponseWriter(w gin.ResponseWriter, maxBytes int) *SessionTraceResponseWriter {
	if maxBytes <= 0 {
		maxBytes = 4 << 20
	}
	return &SessionTraceResponseWriter{
		ResponseWriter: w,
		maxBytes:       maxBytes,
	}
}

func (w *SessionTraceResponseWriter) Write(b []byte) (int, error) {
	w.append(b)
	return w.ResponseWriter.Write(b)
}

func (w *SessionTraceResponseWriter) WriteString(s string) (int, error) {
	w.append([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *SessionTraceResponseWriter) append(b []byte) {
	if w.maxBytes <= 0 || w.buf.Len() >= w.maxBytes {
		return
	}
	remain := w.maxBytes - w.buf.Len()
	if len(b) <= remain {
		_, _ = w.buf.Write(b)
		return
	}
	_, _ = w.buf.Write(b[:remain])
}

func (w *SessionTraceResponseWriter) ResponseBody() string {
	return w.buf.String()
}
