package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

var (
	enc = binary.BigEndian
)

const (
	lenWidth = 8
)

// store holds a records
type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

func newStore(file *os.File) (*store, error) {
	fileStats, err := os.Stat(file.Name())
	if err != nil {
		return nil, err
	}
	size := uint64(fileStats.Size())
	return &store{
		File: file,
		buf:  bufio.NewWriter(file),
		size: size,
	}, nil
}

// Append appends a record to the buffer
func (store *store) Append(p []byte) (n, pos uint64, err error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	pos = store.size
	if err = binary.Write(store.buf, enc, uint64(len(p))); err != nil { // append a record's size of fixed width (8 bytes)
		return 0, 0, err
	}
	w, err := store.buf.Write(p) // append a record
	if err != nil {
		return 0, 0, err
	}
	w += lenWidth // initial w is a num of written bytes, we should increment it for a len of record's size that we appended earlier.
	store.size += uint64(w)
	return uint64(w), pos, nil
}

// Read returns record by position
func (store *store) Read(pos uint64) ([]byte, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if err := store.buf.Flush(); err != nil {
		return nil, err
	} // flush the data from buffer to the file
	size := make([]byte, lenWidth)
	if _, err := store.File.ReadAt(size, int64(pos)); err != nil { // read the first 8 bytes that represents a record's size
		return nil, err
	}

	b := make([]byte, enc.Uint64(size))
	if _, err := store.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	} // read the whole record

	return b, nil
}

// ReadAt reads len(p) bytes of file starting at byte offset off
func (store *store) ReadAt(p []byte, offset int64) (int, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.buf.Flush(); err != nil {
		return 0, err
	}

	return store.File.ReadAt(p, offset)
}

func (store *store) Close() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.buf.Flush(); err != nil {
		return err
	}

	return store.File.Close()
}
