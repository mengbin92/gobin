package commands

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const liveReloadEndpoint = "/_gobin/livereload"

type liveReloadBroker struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newLiveReloadBroker() *liveReloadBroker {
	return &liveReloadBroker{clients: make(map[chan struct{}]struct{})}
}

func (b *liveReloadBroker) subscribe() (chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	if b == nil {
		return ch, func() {}
	}

	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.clients, ch)
		b.mu.Unlock()
		close(ch)
	}
}

func (b *liveReloadBroker) notify() {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (b *liveReloadBroker) serveEvents(w http.ResponseWriter, r *http.Request) {
	if b == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		_, _ = fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()
	}

	events, unsubscribe := b.subscribe()
	defer unsubscribe()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-events:
			_, _ = fmt.Fprint(w, "event: reload\ndata: now\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func newLiveReloadFileHandler(outputDir string, broker *liveReloadBroker) http.Handler {
	fileServer := http.FileServer(http.Dir(outputDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broker == nil || r.Method != http.MethodGet {
			fileServer.ServeHTTP(w, r)
			return
		}

		filePath, ok := liveReloadHTMLPath(outputDir, r.URL.Path)
		if !ok {
			fileServer.ServeHTTP(w, r)
			return
		}

		content, err := injectLiveReloadScript(filePath)
		if err != nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, filepath.Base(filePath), fileModTime(filePath), bytes.NewReader(content))
	})
}

func liveReloadHTMLPath(outputDir, requestPath string) (string, bool) {
	cleanPath := path.Clean("/" + requestPath)
	relPath := strings.TrimPrefix(cleanPath, "/")
	if relPath == "" || relPath == "." {
		relPath = "index.html"
	}

	filePath := filepath.Join(outputDir, filepath.FromSlash(relPath))
	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		filePath = filepath.Join(filePath, "index.html")
		info, err = os.Stat(filePath)
	}
	if err != nil || info.IsDir() || strings.ToLower(filepath.Ext(filePath)) != ".html" {
		return "", false
	}
	return filePath, true
}

func injectLiveReloadScript(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lower := strings.ToLower(string(content))
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		return append(content, liveReloadScript()...), nil
	}

	injected := make([]byte, 0, len(content)+len(liveReloadScript()))
	injected = append(injected, content[:index]...)
	injected = append(injected, liveReloadScript()...)
	injected = append(injected, content[index:]...)
	return injected, nil
}

func liveReloadScript() []byte {
	return []byte(`<script>
(function () {
  if (!window.EventSource) return;
  var source = new EventSource("` + liveReloadEndpoint + `");
  source.addEventListener("reload", function () {
    window.location.reload();
  });
}());
</script>
`)
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
