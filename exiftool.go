package exiftool

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"errors"
)

var binary = "exiftool"
var executeArg = "-execute"
var initArgs = []string{"-stay_open", "True", "-@", "-", "-common_args"}
var extractArgs = []string{"-j"}
var closeArgs = []string{"-stay_open", "False", executeArg}
var readyToken = []byte("{ready}\n")
var readyTokenLen = len(readyToken)

// ErrNotExist is a sentinel error for non existing file
var ErrNotExist = errors.New("file does not exist")

// Exiftool is the exiftool utility wrapper
type Exiftool struct {
	lock       sync.Mutex
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	scanout    *bufio.Scanner
	bufferSize int
}

// NewExiftool instanciates a new Exiftool with configuration functions. If anything went
// wrong, a non empty error will be returned.
func NewExiftool(opts ...func(*Exiftool) error) (*Exiftool, error) {
	e := Exiftool{}

	for _, opt := range opts {
		if err := opt(&e); err != nil {
			return nil, fmt.Errorf("error when configuring exiftool: %w", err)
		}
	}

	cmd := exec.Command(binary, initArgs...)

	var err error
	if e.stdin, err = cmd.StdinPipe(); err != nil {
		return nil, fmt.Errorf("error when piping stdin: %w", err)
	}

	if e.stdout, err = cmd.StdoutPipe(); err != nil {
		return nil, fmt.Errorf("error when piping stdout: %w", err)
	}

	e.scanout = bufio.NewScanner(e.stdout)
	buf := make([]byte, 0, 64*1024)
	e.scanout.Buffer(buf, e.bufferSize)
	e.scanout.Split(splitReadyToken)

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("error when executing commande: %w", err)
	}

	return &e, nil
}

// SetBufferSize changes the buffer size
func (e *Exiftool) SetBufferSize(size int) {
	e.bufferSize = size
}

// Close closes exiftool. If anything went wrong, a non empty error will be returned
func (e *Exiftool) Close() error {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, v := range closeArgs {
		_, err := fmt.Fprintln(e.stdin, v)
		if err != nil {
			return err
		}
	}

	var errs []error
	if err := e.stdout.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error while closing stdout: %w", err))
	}

	if err := e.stdin.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error while closing stdin: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("error while closing exiftool: %w", errs)
	}

	return nil
}

// ExtractMetadata extracts metadata from files
func (e *Exiftool) ExtractMetadata(files ...string) []FileMetadata {
	e.lock.Lock()
	defer e.lock.Unlock()

	fms := make([]FileMetadata, len(files))

	for i, f := range files {
		fms[i].File = f

		if _, err := os.Stat(f); err != nil {
			if os.IsNotExist(err) {
				fms[i].Err = ErrNotExist
				continue
			}

			fms[i].Err = err

			continue
		}

		for _, curA := range extractArgs {
			fmt.Fprintln(e.stdin, curA)
		}

		fmt.Fprintln(e.stdin, f)
		fmt.Fprintln(e.stdin, executeArg)

		if !e.scanout.Scan() {
			fms[i].Err = fmt.Errorf("nothing on stdout")
			continue
		}

		if e.scanout.Err() != nil {
			fms[i].Err = fmt.Errorf("error while reading stdout: %w", e.scanout.Err())
			continue
		}

		var m []map[string]interface{}
		if err := json.Unmarshal(e.scanout.Bytes(), &m); err != nil {
			fms[i].Err = fmt.Errorf("error during unmarshaling (%v): %w)", e.scanout.Bytes(), err)
			continue
		}

		fms[i].Fields = m[0]
	}

	return fms
}

func splitReadyToken(data []byte, atEOF bool) (int, []byte, error) {
	idx := bytes.Index(data, readyToken)
	if idx == -1 {
		if atEOF && len(data) > 0 {
			return 0, data, fmt.Errorf("no final token found")
		}

		return 0, nil, nil
	}

	return idx + readyTokenLen, data[:idx], nil
}
