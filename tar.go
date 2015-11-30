package garchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Compression is the state represtents if compressed or not.
type Compression int

const (
	// Uncompressed represents the uncompressed.
	Uncompressed Compression = iota
	// Gzip is gzip compression algorithm.
	Gzip
)

// TarFile represents a tar file in disk.
// It can be compressed (gzip) or not.
type TarFile struct {
	Filename       string
	Compression    Compression
	file           *os.File
	writer         *tar.Writer
	compressWriter io.WriteCloser
}

// TarAddOptions ...
type TarAddOptions struct {
	IncludeSourceDir bool // Only used when the path is a directory
	Recursive        bool
}

// NewTarFile creates a new tar file on disk.
func NewTarFile(name string, compression Compression) (*TarFile, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	return &TarFile{Filename: name, Compression: compression, file: file}, nil
}

// OpenTarFile opens a tar file on disk.
func OpenTarFile(name string) (*TarFile, error) {
	file, err := os.OpenFile(name, os.O_RDWR, os.ModePerm)

	if err != nil {
		return nil, err
	}

	compression := detectCompression(file)

	return &TarFile{Filename: name, Compression: compression, file: file}, nil
}

// Close closes the tar file., flushing any unwritten
// data to the underlying writer.
func (t *TarFile) Close() error {
	if err := t.writer.Close(); err != nil {
		return err
	}

	if t.compressWriter != nil {
		if err := t.compressWriter.Close(); err != nil {
			return err
		}
	}

	return t.file.Close()
}

// Add adds a file or a directory into tar file.
func (t *TarFile) Add(name string, options *TarAddOptions) error {
	if options == nil {
		options = &TarAddOptions{Recursive: true}
	}

	// Removes the last slash to avoid different behaviors when `name` is a folder
	name = strings.TrimSuffix(name, "/")

	fileInfo, err := os.Stat(name)
	if err != nil {
		return err
	}

	baseDir := path.Dir(name)

	if fileInfo.IsDir() && !options.IncludeSourceDir {
		baseDir = name
	}

	return t.write(name, fileInfo, baseDir, options.Recursive)
}

func (t *TarFile) write(name string, fileInfo os.FileInfo, baseDir string, recursive bool) error {
	t.ensureWriter()

	header, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return err
	}

	if header.Name, err = filepath.Rel(baseDir, name); err != nil {
		return err
	}

	// It happens when `name` is a folder and IncludeSourceDir is false
	if header.Name != "." {
		if err := t.writer.WriteHeader(header); err != nil {
			return err
		}
	}

	if fileInfo.IsDir() {
		fileInfos, err := ioutil.ReadDir(name)
		if err != nil {
			return err
		}

		for _, fileInfo := range fileInfos {
			if !recursive && fileInfo.IsDir() {
				continue
			}

			err := t.write(path.Join(name, fileInfo.Name()), fileInfo, baseDir, recursive)
			if err != nil {
				return err
			}
		}
	} else {
		file, err := os.Open(name)
		if err != nil {
			return err
		}

		defer file.Close()

		_, err = io.Copy(t.writer, file)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *TarFile) ensureWriter() {
	if t.writer != nil {
		return
	}

	if t.Compression == Gzip {
		t.compressWriter = gzip.NewWriter(t.file)
	}

	if t.compressWriter == nil {
		t.writer = tar.NewWriter(t.file)
	} else {
		t.writer = tar.NewWriter(t.compressWriter)
	}
}

func detectCompression(file *os.File) Compression {
	source := make([]byte, 4)

	file.Read(source)

	for compression, m := range map[Compression][]byte{
		Gzip: {0x1F, 0x8B, 0x08},
	} {
		if len(source) < len(m) {
			continue
		}
		if bytes.Compare(m, source[:len(m)]) == 0 {
			return compression
		}
	}
	return Uncompressed
}
