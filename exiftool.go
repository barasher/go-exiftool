package exiftool

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

var executeArg = "-execute"
var initArgs = []string{"-stay_open", "True", "-@", "-"}
var extractArgs = []string{"-j"}
var closeArgs = []string{"-stay_open", "False", executeArg}
var readyTokenLen = len(readyToken)

// WaitTimeout specifies the duration to wait for exiftool to exit when closing before timing out
var WaitTimeout = time.Second

// ErrNotExist is a sentinel error for non existing file
var ErrNotExist = errors.New("file does not exist")

// Exiftool is the exiftool utility wrapper
type Exiftool struct {
	lock            sync.Mutex
	stdin           io.WriteCloser
	stdMergedOut    io.ReadCloser
	scanMergedOut   *bufio.Scanner
	bufferSet       bool
	buffer          []byte
	bufferMaxSize   int
	extraInitArgs   []string
	exiftoolBinPath string
	cmd             *exec.Cmd
}

// NewExiftool instanciates a new Exiftool with configuration functions. If anything went
// wrong, a non empty error will be returned.
func NewExiftool(opts ...func(*Exiftool) error) (*Exiftool, error) {
	e := Exiftool{
		exiftoolBinPath: exiftoolBinary,
	}

	for _, opt := range opts {
		if err := opt(&e); err != nil {
			return nil, fmt.Errorf("error when configuring exiftool: %w", err)
		}
	}

	args := append([]string(nil), initArgs...)
	if len(e.extraInitArgs) > 0 {
		args = append(args, "-common_args")
		args = append(args, e.extraInitArgs...)
	}

	e.cmd = exec.Command(e.exiftoolBinPath, args...)
	r, w := io.Pipe()
	e.stdMergedOut = r

	e.cmd.Stdout = w
	e.cmd.Stderr = w

	var err error
	if e.stdin, err = e.cmd.StdinPipe(); err != nil {
		return nil, fmt.Errorf("error when piping stdin: %w", err)
	}

	e.scanMergedOut = bufio.NewScanner(r)
	if e.bufferSet {
		e.scanMergedOut.Buffer(e.buffer, e.bufferMaxSize)
	}
	e.scanMergedOut.Split(splitReadyToken)

	if err = e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("error when executing command: %w", err)
	}

	return &e, nil
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
	if err := e.stdMergedOut.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error while closing stdMergedOut: %w", err))
	}

	if err := e.stdin.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error while closing stdin: %w", err))
	}

	ch := make(chan struct{})
	go func() {
		if e.cmd != nil {
			if err := e.cmd.Wait(); err != nil {
				errs = append(errs, fmt.Errorf("error while waiting for exiftool to exit: %w", err))
			}
		}
		ch <- struct{}{}
		close(ch)
	}()

	// Wait for wait to finish or timeout
	select {
	case <- ch:
	case <- time.After(WaitTimeout):
		errs = append(errs, errors.New("Timed out waiting for exiftool to exit"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("error while closing exiftool: %v", errs)
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

		if !e.scanMergedOut.Scan() {
			fms[i].Err = fmt.Errorf("nothing on stdMergedOut")
			continue
		}

		if e.scanMergedOut.Err() != nil {
			fms[i].Err = fmt.Errorf("error while reading stdMergedOut: %w", e.scanMergedOut.Err())
			continue
		}

		var m []map[string]interface{}
		if err := json.Unmarshal(e.scanMergedOut.Bytes(), &m); err != nil {
			fms[i].Err = fmt.Errorf("error during unmarshaling (%v): %w)", string(e.scanMergedOut.Bytes()), err)
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

// Buffer defines the buffer used to read from stdout and stderr, see https://golang.org/pkg/bufio/#Scanner.Buffer
// Sample :
//  buf := make([]byte, 128*1000)
//  e, err := NewExiftool(Buffer(buf, 64*1000))
func Buffer(buf []byte, max int) func(*Exiftool) error {
	return func(e *Exiftool) error {
		e.bufferSet = true
		e.buffer = buf
		e.bufferMaxSize = max
		return nil
	}
}

// Charset defines the -charset value to pass to Exiftool, see https://exiftool.org/faq.html#Q10 and https://exiftool.org/faq.html#Q18
// Sample :
//   e, err := NewExiftool(Charset("filename=utf8"))
func Charset(charset string) func(*Exiftool) error {
	return func(e *Exiftool) error {
		e.extraInitArgs = append(e.extraInitArgs, "-charset", charset)
		return nil
	}
}

// NoPrintConversion enables 'No print conversion' mode, see https://exiftool.org/exiftool_pod.html.
// Sample :
//   e, err := NewExiftool(NoPrintConversion())
func NoPrintConversion() func(*Exiftool) error {
	return func(e *Exiftool) error {
		e.extraInitArgs = append(e.extraInitArgs, "-n")
		return nil
	}
}

// ExtractEmbedded extracts embedded metadata from files (activates Exiftool's '-ee' paramater)
// Sample :
//   e, err := NewExiftool(ExtractEmbedded())
func ExtractEmbedded() func(*Exiftool) error {
	return func(e *Exiftool) error {
		e.extraInitArgs = append(e.extraInitArgs, "-ee")
		return nil
	}
}

// ExtractAllBinaryMetadata extracts all binary metadata (activates Exiftool's '-b' paramater)
// Sample :
//   e, err := NewExiftool(ExtractAllBinaryMetadata())
func ExtractAllBinaryMetadata() func(*Exiftool) error {
	return func(e *Exiftool) error {
		e.extraInitArgs = append(e.extraInitArgs, "-b")
		return nil
	}
}

// SetExiftoolBinaryPath sets exiftool's binary path. When not specified, the binary will have to be in $PATH
// Sample :
//   e, err := NewExiftool(SetExiftoolBinaryPath("/usr/bin/exiftool"))
func SetExiftoolBinaryPath(p string) func(*Exiftool) error {
	return func(e *Exiftool) error {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("error while checking if path '%v' exists: %w", p, err)
		}
		e.exiftoolBinPath = p
		return nil
	}
}
