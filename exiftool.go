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

	"github.com/pkg/errors"
)

// ErrNotExist is a sentinel error for non existing file
var ErrNotExist = errors.New("file does not exist")

// Config contains the configuration used by the module
type Config struct {
	binary        string
	executeArg    string
	initArgs      []string
	extractArgs   []string
	closeArgs     []string
	readyToken    []byte
	readyTokenLen int
}

func NewExiftoolConfig(opts ...func(*Config) error) (*Config, error) {
	config := Config{
		binary:        "exiftool",
		executeArg:    "-execute",
		initArgs:      []string{"-stay_open", "True", "-@", "-", "-common_args"},
		extractArgs:   []string{"-j"},
		closeArgs:     nil,
		readyToken:    []byte("{ready}\n"),
		readyTokenLen: 0,
	}

	config.closeArgs = []string{"-stay_open", "False", config.executeArg}
	config.readyTokenLen = len(config.readyToken)

	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return nil, errors.Wrap(err, "Error setting configuration options")
		}
	}

	return &config, nil
}

// Exiftool is the exiftool utility wrapper
type Exiftool struct {
	config  *Config
	lock    sync.Mutex
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanout *bufio.Scanner
}

// NewExiftool instanciates a new Exiftool with configuration functions. If anything went
// wrong, a non empty error will be returned. config is nil, it will use a default config
func NewExiftool(config *Config, opts ...func(*Exiftool) error) (*Exiftool, error) {
	e := Exiftool{}

	for _, opt := range opts {
		if err := opt(&e); err != nil {
			return nil, fmt.Errorf("error when configuring exiftool: %w", err)
		}
	}

	if e.config == nil {
		defaultConfig, err := NewExiftoolConfig()
		if err != nil {
			return nil, err
		}

		e.config = defaultConfig
	}

	cmd := exec.Command(e.config.binary, e.config.initArgs...)

	var err error
	if e.stdin, err = cmd.StdinPipe(); err != nil {
		return nil, fmt.Errorf("error when piping stdin: %w", err)
	}

	if e.stdout, err = cmd.StdoutPipe(); err != nil {
		return nil, fmt.Errorf("error when piping stdout: %w", err)
	}

	e.scanout = bufio.NewScanner(e.stdout)
	e.scanout.Split(splitReadyToken(e.config.readyToken))

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("error when executing commande: %w", err)
	}

	return &e, nil
}

// Close closes exiftool. If anything went wrong, a non empty error will be returned
func (e *Exiftool) Close() error {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, v := range e.config.closeArgs {
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
			if err == os.ErrNotExist {
				fms[i].Err = ErrNotExist
			} else {
				fms[i].Err = err
			}

			continue
		}

		for _, curA := range e.config.extractArgs {
			fmt.Fprintln(e.stdin, curA)
		}

		fmt.Fprintln(e.stdin, f)
		fmt.Fprintln(e.stdin, e.config.executeArg)

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

func splitReadyToken(readyToken []byte) func(data []byte, atEOF bool) (int, []byte, error) {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		idx := bytes.Index(data, readyToken)
		if idx == -1 {
			if atEOF && len(data) > 0 {
				return 0, data, fmt.Errorf("no final token found")
			}

			return 0, nil, nil
		}

		return idx + len(readyToken), data[:idx], nil
	}
}
