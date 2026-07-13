package document

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// PipelineOption configures the Pipeline.
type PipelineOption func(*Pipeline)

// WithEmitter sets an optional observability emitter for document pipeline events.
func WithEmitter(emitter interface{}) PipelineOption {
	return func(p *Pipeline) {
		p.emitter = emitter
	}
}

// WithMaxConcurrency sets the maximum number of concurrent parsing goroutines.
func WithMaxConcurrency(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.maxConcurrency = n
		}
	}
}

// WithRegistry sets a custom parser registry. If nil, the default
// registry (all built-in parsers) is used.
func WithRegistry(r *ParserRegistry) PipelineOption {
	return func(p *Pipeline) {
		if r != nil {
			p.registry = r
		}
	}
}

// Pipeline orchestrates concurrent document parsing across multiple parsers.
type Pipeline struct {
	registry       *ParserRegistry
	maxConcurrency int
	emitter        interface{} // *observability.Emitter
}

// NewPipeline creates a Pipeline with sensible defaults and registers all built-in parsers.
func NewPipeline(opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		registry:       NewParserRegistry(),
		maxConcurrency: 5,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Register adds a parser to the pipeline's registry.
func (p *Pipeline) Register(parser Parser) {
	p.registry.Register(parser)
}

// ParseAll parses all files concurrently and returns results plus stats.
// targetDir is the upload root (contains docs/ and assets/ subdirectories).
// uploadID is used for per-file structured log entries.
func (p *Pipeline) ParseAll(ctx context.Context, files []File, targetDir string, uploadID string) ([]ParsedDocument, ParseStats) {
	startTime := time.Now()
	if len(files) == 0 {
		return nil, ParseStats{}
	}

	sem := make(chan struct{}, p.maxConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	results := make([]ParsedDocument, 0, len(files))

	for _, f := range files {
		select {
		case <-ctx.Done():
			wg.Wait()
			log.Printf("document pipeline: context cancelled, processed %d of %d files",
				len(results), len(files))
			stats := buildStats(results, time.Since(startTime))
			return results, stats
		default:
		}

		wg.Add(1)
		f := f
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			logParseStart(uploadID, f.Name, f.Size)
			t0 := time.Now()

			parser := p.resolveParser(f.Name, f.Data)
			doc, err := parser.Parse(f, targetDir)
			if err != nil {
				doc = &ParsedDocument{
					OriginalName: f.Name,
					Type:         "error",
					Error:        err.Error(),
				}
			}

			dur := time.Since(t0).Milliseconds()
			if doc.Error != "" {
				logParseError(uploadID, f.Name, doc.Error)
			} else {
				logParseComplete(uploadID, f.Name, doc.Type, doc.CharCount, dur)
			}

			mu.Lock()
			results = append(results, *doc)
			mu.Unlock()
		}()
	}

	wg.Wait()

	stats := buildStats(results, time.Since(startTime))

	log.Printf("document pipeline: parsed %d files, %d succeeded, %d unsupported, %d errors",
		len(files), stats.Succeeded, stats.Unsupported, stats.Errors)

	return results, stats
}

func buildStats(docs []ParsedDocument, totalDur time.Duration) ParseStats {
	return ParseStats{
		TotalFiles:      len(docs),
		Succeeded:       countSuccess(docs),
		Unsupported:     countUnsupported(docs),
		Errors:          countErrors(docs),
		TotalDurationMs: totalDur.Milliseconds(),
	}
}

func logParseStart(uploadID, fileName string, size int64) {
	entry := map[string]any{
		"event":     "parse_start",
		"upload_id": uploadID,
		"file":      fileName,
		"size":      size,
	}
	b, _ := json.Marshal(entry)
	log.Print(string(b))
}

func logParseComplete(uploadID, fileName, fileType string, charCount int, durationMs int64) {
	entry := map[string]any{
		"event":      "parse_complete",
		"upload_id":  uploadID,
		"file":       fileName,
		"type":       fileType,
		"char_count": charCount,
		"duration_ms": durationMs,
	}
	b, _ := json.Marshal(entry)
	log.Print(string(b))
}

func logParseError(uploadID, fileName, errMsg string) {
	entry := map[string]any{
		"event":     "parse_error",
		"upload_id": uploadID,
		"file":      fileName,
		"error":     errMsg,
	}
	b, _ := json.Marshal(entry)
	log.Print(string(b))
}

// resolveParser finds the appropriate parser for a file.
func (p *Pipeline) resolveParser(name string, data []byte) Parser {
	return p.registry.Resolve(name, data)
}

func countSuccess(docs []ParsedDocument) int {
	n := 0
	for _, d := range docs {
		if d.Error == "" && d.Type != "unsupported" {
			n++
		}
	}
	return n
}

func countUnsupported(docs []ParsedDocument) int {
	n := 0
	for _, d := range docs {
		if d.Type == "unsupported" {
			n++
		}
	}
	return n
}

func countErrors(docs []ParsedDocument) int {
	n := 0
	for _, d := range docs {
		if d.Error != "" {
			n++
		}
	}
	return n
}
