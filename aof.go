package main

import (
	"bufio"
	"io"
	"os"
	"sync"
	"time"
)

// Aof represents an append-only file for persisting Redis commands.
type Aof struct {
	file *os.File
	rd   *bufio.Reader
	mu   sync.Mutex
}

// NewAOF creates or opens a new AOF file and starts a background sync goroutine.
func NewAOF(path string) (*Aof, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	aof := &Aof{
		file: file,
		rd:   bufio.NewReader(file),
	}

	// Start a goroutine to sync AOF to disk every 3 seconds
	go func() {
		for {
			aof.mu.Lock()
			aof.file.Sync()
			aof.mu.Unlock()
			time.Sleep(3 * time.Second)
		}
	}()

	return aof, nil
}

// Close flushes and closes the AOF file.
func (aof *Aof) Close() error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	return aof.file.Close()
}

// Write appends a marshaled RESP value to the AOF file.
func (aof *Aof) Write(value Value) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	_, err := aof.file.Write(value.Marshal())
	if err != nil {
		return err
	}

	return nil
}

// Read reads all commands from the AOF file and executes the callback for each.
func (aof *Aof) Read(callback func(value Value)) error {
	aof.mu.Lock()
	defer aof.mu.Unlock()

	aof.file.Seek(0, io.SeekStart) // Reset to beginning of file
	resp := NewResp(aof.file)

	for {
		value, err := resp.Read()
		if err != nil {
			if err == io.EOF {
				break // End of file, done reading
			}
			return err // Real error occurred
		}

		callback(value) // Process the command
	}

	return nil
}
