package log

import (
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

var (
	offWidth uint64 = 4
	posWidth uint64 = 8
	endWidth        = offWidth + posWidth // total width of record is 12 bytes
)

// index holds offset and position of the record in a store file
type index struct {
	file *os.File    // relative file
	mmap gommap.MMap // in-memory map
	size uint64
}

// newIndex creates a file with fixed size and connects it to in-memory map
func newIndex(file *os.File, c Config) (*index, error) {
	idx := &index{
		file: file,
	}

	stats, err := os.Stat(file.Name())
	if err != nil {
		return nil, err
	}
	idx.size = uint64(stats.Size())
	if err = os.Truncate(file.Name(), int64(c.Segment.MaxIndexBytes)); err != nil {
		return nil, err
	} // fix a size of file according to MaxIndexBytes
	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, err
	}

	return idx, nil
}

// Close closes index's underlying file and in-memory map
func (index *index) Close() error {
	if err := index.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	} // flush the data to persisted file
	if err := index.file.Sync(); err != nil {
		return err
	} // flush the data to stable storage
	if err := index.file.Truncate(int64(index.size)); err != nil {
		return err
	} // erase empty space in the end of file

	return index.file.Close()
}

// Read reads a data in the index file by offset in.
// Returns relative offset, position and error.
func (index *index) Read(in int64) (out uint32, pos uint64, err error) {
	if index.size == 0 {
		return 0, 0, io.EOF
	}
	if in == -1 {
		out = uint32((index.size / endWidth) - 1) // last record offset
	} else {
		out = uint32(in)
	}
	pos = uint64(out) * endWidth // if there is 10 records 12 bytes each, and we need the 6th record, then 6*12 will be position that we are looking for
	if index.size < pos+endWidth {
		return 0, 0, io.EOF
	}
	out = enc.Uint32(index.mmap[pos : pos+offWidth])          // 6*12:6*12+4 is an offset with 4 bytes len
	pos = enc.Uint64(index.mmap[pos+offWidth : pos+endWidth]) // 6*12+4:6*12+12 is a position with 8 bytes len
	return out, pos, nil
}

// Write writes records offset and position into in-memory map
func (index *index) Write(off uint32, pos uint64) error {
	if uint64(len(index.mmap)) < index.size+endWidth { // if there is not enough space in map
		return io.EOF
	}
	enc.PutUint32(index.mmap[index.size:index.size+offWidth], off)          // write 4 bytes of offset
	enc.PutUint64(index.mmap[index.size+offWidth:index.size+endWidth], pos) // and 8 bytes of position
	index.size += endWidth                                                  // increase index's size on 4+8 bytes
	return nil
}

// Name returns name of underlying file
func (index *index) Name() string {
	return index.file.Name()
}
