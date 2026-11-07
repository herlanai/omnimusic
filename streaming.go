package main

import (
	"context"
	"crypto/sha1"
	encodinghex "encoding/hex"
	"errors"
	"io"
	"omnimusic/locales"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

type transferProgress struct {
	bytes   atomic.Int64
	failure atomic.Pointer[error]
}

// Read and seek use logical offsets. Reads beyond the downloaded prefix sleep
// until the writer publishes more bytes, fails, or is cancelled.
type streamingFile struct {
	mu                   sync.Mutex
	changed              *sync.Cond
	file                 *os.File
	size, available, pos int64
	finished, closed     bool
	err                  error
	cancel               context.CancelFunc
	done                 chan struct{}
}

func beginTransfer(body io.ReadCloser, t track, path string, progress *transferProgress, cancel context.CancelFunc) (*streamingFile, error) {
	tmp, e := os.CreateTemp(filepath.Dir(path), "download-*")
	if e != nil {
		return nil, e
	}
	s := &streamingFile{file: tmp, size: t.Size, cancel: cancel, done: make(chan struct{})}
	s.changed = sync.NewCond(&s.mu)
	go s.download(body, t, path, progress)
	return s, nil
}
func (s *streamingFile) download(body io.ReadCloser, t track, path string, progress *transferProgress) {
	defer close(s.done)
	defer body.Close()
	hash := sha1.New()
	buffer := make([]byte, 32*1024)
	var offset int64
	var err error
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			if int64(n) > s.size-offset {
				err = errors.New(locales.LengthMismatch())
				break
			}
			if _, err = s.file.WriteAt(buffer[:n], offset); err != nil {
				break
			}
			hash.Write(buffer[:n])
			offset += int64(n)
			s.mu.Lock()
			s.available = offset
			s.changed.Broadcast()
			s.mu.Unlock()
			progress.bytes.Store(offset)
		}
		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}
			break
		}
	}
	if err == nil && (offset != t.Size || encodinghex.EncodeToString(hash.Sum(nil)) != t.Digest) {
		err = errors.New(locales.Checksum())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tempName := s.file.Name()
	// Close before renaming so the same finalization also works on Windows.
	if closeErr := s.file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tempName, path)
		if err != nil {
			if st, e := os.Stat(path); e == nil && st.Size() == s.size {
				err = nil
			}
		}
		if err == nil && !s.closed {
			s.file, err = os.Open(path)
		}
	}
	os.Remove(tempName)
	s.err = err
	s.finished = true
	if err != nil {
		progress.failure.Store(&err)
	}
	s.changed.Broadcast()
}
func (s *streamingFile) Read(dst []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(dst) == 0 {
		return 0, nil
	}
	for !s.closed && !s.finished && s.pos >= s.available {
		s.changed.Wait()
	}
	if s.closed {
		return 0, os.ErrClosed
	}
	if s.err != nil {
		return 0, s.err
	}
	if s.pos >= s.available {
		return 0, io.EOF
	}
	n, e := s.file.ReadAt(dst[:min(int64(len(dst)), s.available-s.pos)], s.pos)
	s.pos += int64(n)
	return n, e
}
func (s *streamingFile) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, os.ErrClosed
	}
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += s.pos
	case io.SeekEnd:
		offset += s.size
	default:
		return 0, errors.New(locales.InvalidSeek())
	}
	if offset < 0 {
		return 0, errors.New(locales.InvalidOffset())
	}
	s.pos = offset
	return offset, nil
}
func (s *streamingFile) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.changed.Broadcast()
	s.mu.Unlock()
	s.cancel()
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil && s.err == nil {
		_ = s.file.Close()
	}
	return nil
}
