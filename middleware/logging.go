package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Alge/tillit/requestdata"
)

type wrappedWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *wrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

func (w *wrappedWriter) Write(b []byte) (int, error) {
	// Write the data to the underlying ResponseWriter
	bytes, err := w.ResponseWriter.Write(b)

	// Accumulate the number of bytes written
	w.bytesWritten += bytes

	return bytes, err
}

func humanReadableBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func Logging(next http.Handler) http.Handler {

	readUserIP := func(r *http.Request) string {
		IPAddress := r.Header.Get("X-Real-Ip")
		if IPAddress == "" {
			IPAddress = r.Header.Get("X-Forwarded-For")
		}
		if IPAddress == "" {
			IPAddress = r.RemoteAddr
		}
		return IPAddress
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rId, ok := requestdata.GetRequestID(r)
		var rIdString string
		if ok {
			rIdString = rId.String()[:8]
		} else {
			rIdString = "Unknown"
		}

		log.Printf("[%s] %s %s %s %s", rIdString, r.Method, r.URL.Path, r.URL.RawQuery, readUserIP(r))

		wrapped := &wrappedWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		log.Printf("[%s] %d %s %s %s", rIdString, wrapped.statusCode, humanReadableBytes(wrapped.bytesWritten), time.Since(start), r.URL.Path)
	})
}
