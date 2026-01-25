package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type entry struct {
	logger *log.Logger
	writer io.Writer
}

var (
	mu          sync.Mutex
	initialized bool
	logDir      string
	loggers     map[string]*entry
)

// Init configures logging directory and sets the standard logger to the app log.
func Init(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	logDir = dir
	loggers = make(map[string]*entry)
	initialized = true

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	appEntry, err := buildLoggerLocked("app")
	if err != nil {
		log.SetOutput(timeWriter{w: os.Stdout})
		log.SetFlags(0)
		return err
	}

	loggers["app"] = appEntry
	log.SetOutput(appEntry.writer)
	log.SetFlags(0)
	return nil
}

func Get(name string) *log.Logger {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return log.New(timeWriter{w: os.Stdout}, "", 0)
	}

	if entry := loggers[name]; entry != nil {
		return entry.logger
	}

	entry, err := buildLoggerLocked(name)
	if err != nil {
		return log.New(timeWriter{w: os.Stdout}, fmt.Sprintf("[%s] ", name), 0)
	}
	loggers[name] = entry
	return entry.logger
}

func buildLoggerLocked(name string) (*entry, error) {
	suffix := time.Now().Format("06.01") // yy.mm
	filename := suffix + "_" + name + ".log"
	path := filepath.Join(logDir, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	writer := io.MultiWriter(os.Stdout, file)
	tw := timeWriter{w: writer}
	logger := log.New(tw, "", 0)
	return &entry{logger: logger, writer: writer}, nil
}

type timeWriter struct {
	w io.Writer
}

func (t timeWriter) Write(p []byte) (int, error) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	lines := strings.Split(string(p), "\n")
	total := 0
	for i, line := range lines {
		if line == "" && i == len(lines)-1 {
			continue
		}
		entry := ts + ", " + line
		if i < len(lines)-1 {
			entry += "\n"
		}
		n, err := t.w.Write([]byte(entry))
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
