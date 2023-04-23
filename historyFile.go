package readline

import (
	"bufio"
	"io"
	"os"
	"strings"
	"sync"
)

type historyFile struct {
	fn    string
	fd    *os.File
	limit int
	mut   sync.Mutex
}

func NewHistoryFile(fn string, limit int) *historyFile {
	return &historyFile{
		fn:    fn,
		limit: limit,
	}
}

func (hf *historyFile) Load() ([][]rune, error) {
	hf.mut.Lock()
	defer hf.mut.Unlock()

	lines, total, err := _load(hf.fn, hf.limit)

	if err == nil && hf.limit > 0 && total > hf.limit {
		err = _rewrite(hf.fn, lines)
	}

	return lines, err
}

func (hf *historyFile) Append(line []rune) (err error) {
	hf.mut.Lock()
	defer hf.mut.Unlock()

	if err = hf.openAppendOnly(); err != nil {
		return
	}

	// Single write here in case muliple processes
	// are appending to this file.
	data := strings.TrimSpace(string(line)) + "\n"
	_, err = hf.fd.Write([]byte(data))
	return
}

func (hf *historyFile) Close() (err error) {
	hf.mut.Lock()
	defer hf.mut.Unlock()

	if hf.fd == nil {
		return
	}

	if err = hf.fd.Close(); err == nil {
		hf.fd = nil
	}

	return
}

// expect lock to be held
func (hf *historyFile) openAppendOnly() error {
	if hf.fd != nil {
		return nil
	}
	fd, err := os.OpenFile(hf.fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	hf.fd = fd
	return nil
}

func _load(fn string, limit int) ([][]rune, int, error) {
	var err error

	fd, err := os.Open(fn)
	if err != nil {
		return nil, 0, err
	}
	defer fd.Close()

	total := 0
	var lines [][]rune
	r := bufio.NewReader(fd)

	for ; ; total++ {
		var line string
		line, err = r.ReadString('\n')
		if err != nil {
			break
		}
		// ignore the empty line
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		lines = append(lines, []rune(line))

		if limit > 0 && len(lines) > limit {
			lines = lines[1:]
		}
	}

	if err == io.EOF {
		err = nil
	}

	return lines, total, err
}

func _rewrite(fn string, lines [][]rune) (err error) {

	tmpFile := fn + ".tmp"
	fd, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_APPEND, 0666)
	if err != nil {
		return
	}

	defer func() {
		fd.Close()
		if err != nil {
			os.Remove(tmpFile) // Try to cleanup dead temp file
		}
	}()

	buf := bufio.NewWriter(fd)
	for _, line := range lines {
		if _, err = buf.WriteString(string(line)); err != nil {
			return
		}
		if err = buf.WriteByte('\n'); err != nil {
			return
		}
	}

	if err = buf.Flush(); err != nil {
		return
	}

	return os.Rename(tmpFile, fn)
}
