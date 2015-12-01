package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
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

var (
	// ErrPathNotFound returned when a path is not found.
	ErrPathNotFound = errors.New("Path not found.")
)

// TarFile represents a tar file in disk.
// It can be compressed (gzip) or not.
type TarFile struct {
	Name        string
	Compression Compression
	file        *os.File
	writer      *tarWriter
	reader      *tarReader
	entries     []*tar.Header
	loaded      bool
}

// TarAddOptions ...
type TarAddOptions struct {
	IncludeSourceDir bool // Only used when the path is a directory
}

type tarReader struct {
	tarReader      *tar.Reader
	compressReader io.ReadCloser
}

func (r *tarReader) Next() (*tar.Header, error) {
	return r.tarReader.Next()
}

func (r *tarReader) Read(b []byte) (int, error) {
	return r.tarReader.Read(b)
}

func (r *tarReader) Close() error {
	if r.compressReader != nil {
		return r.compressReader.Close()
	}
	return nil
}

type tarWriter struct {
	tarWriter      *tar.Writer
	compressWriter io.WriteCloser
}

func (w *tarWriter) Write(b []byte) (n int, err error) {
	return w.tarWriter.Write(b)
}

func (w *tarWriter) WriteHeader(hdr *tar.Header) error {
	return w.tarWriter.WriteHeader(hdr)
}

func (w *tarWriter) Close() error {
	if err := w.tarWriter.Close(); err != nil {
		return err
	}

	if w.compressWriter != nil {
		return w.compressWriter.Close()
	}

	return nil
}

func newTarReader(tarfile *TarFile) (reader *tarReader, err error) {
	reader = &tarReader{}

	if tarfile.Compression == Gzip {
		reader.compressReader, err = gzip.NewReader(tarfile.file)
		if err != nil {
			return nil, err
		}
	}

	if reader.compressReader == nil {
		reader.tarReader = tar.NewReader(tarfile.file)
	} else {
		reader.tarReader = tar.NewReader(reader.compressReader)
	}

	return
}

func newTarWriter(tarfile *TarFile) *tarWriter {
	writer := &tarWriter{}

	if tarfile.Compression == Gzip {
		writer.compressWriter = gzip.NewWriter(tarfile.file)
	}

	if writer.compressWriter == nil {
		writer.tarWriter = tar.NewWriter(tarfile.file)
	} else {
		writer.tarWriter = tar.NewWriter(writer.compressWriter)
	}

	return writer
}

func getTarCompression(file *os.File) (Compression, error) {
	source := make([]byte, 4)

	if _, err := file.Read(source); err != nil {
		return Uncompressed, err
	}

	if _, err := file.Seek(0, os.SEEK_SET); err != nil {
		return Uncompressed, err
	}

	for compression, m := range map[Compression][]byte{
		Gzip: {0x1F, 0x8B, 0x08},
	} {
		if len(source) < len(m) {
			continue
		}
		if bytes.Compare(m, source[:len(m)]) == 0 {
			return compression, nil
		}
	}
	return Uncompressed, nil
}

// NewTarFile creates a new tar file on disk.
func NewTarFile(name string, compression Compression) (*TarFile, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	tarfile := &TarFile{
		Name:        name,
		Compression: compression,
		file:        file,
		entries:     []*tar.Header{}}

	tarfile.writer = newTarWriter(tarfile)
	return tarfile, nil
}

// OpenTarFile opens a tar file on disk.
func OpenTarFile(name string) (*TarFile, error) {
	file, err := os.OpenFile(name, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	compression, err := getTarCompression(file)
	if err != nil {
		return nil, err
	}

	tarfile := &TarFile{
		Name:        name,
		Compression: compression,
		file:        file,
		entries:     []*tar.Header{}}

	tarfile.reader, err = newTarReader(tarfile)
	if err != nil {
		return nil, err
	}

	return tarfile, nil
}

// AppendTarFile opens a tar file on disk to append new files.
func AppendTarFile(name string) (*TarFile, error) {
	file, err := os.OpenFile(name, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	compression, err := getTarCompression(file)
	if err != nil {
		return nil, err
	}

	if compression != Uncompressed {
		return nil, errors.New("Append is not supported for compressed tar.")
	}

	if _, err = file.Seek(-2<<9, os.SEEK_END); err != nil {
		return nil, err
	}

	tarfile := &TarFile{
		Name:        name,
		Compression: compression,
		file:        file,
		entries:     []*tar.Header{}}

	tarfile.writer = newTarWriter(tarfile)
	return tarfile, nil
}

// Close closes the tar file., flushing any unwritten
// data to the underlying writer.
func (t *TarFile) Close() error {
	if t.writer != nil {
		if err := t.writer.Close(); err != nil {
			return err
		}
	}

	if t.reader != nil {
		if err := t.reader.Close(); err != nil {
			return err
		}
	}

	return t.file.Close()
}

// Add adds a file or a directory into tar file.
// Parameter `name` is the path (file or directory) to be added.
func (t *TarFile) Add(name string, options *TarAddOptions) error {
	if options == nil {
		options = &TarAddOptions{}
	}

	fileInfo, err := os.Stat(name)
	if err != nil {
		return err
	}

	// Removes the last slash to avoid different behaviors when `name` is a folder
	name = strings.TrimSuffix(name, string(os.PathSeparator))

	baseDir := path.Dir(name)

	if fileInfo.IsDir() && !options.IncludeSourceDir {
		baseDir = name
	}

	return filepath.Walk(name,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			if header.Name, err = filepath.Rel(baseDir, path); err != nil {
				return err
			}

			if header.Name == "." {
				return nil
			}

			if err := t.writer.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}

			defer file.Close()

			_, err = io.Copy(t.writer, file)
			return err
		})
}

// GetNames returns the entries as a list of their names.
func (t *TarFile) GetNames() ([]string, error) {
	entries, err := t.GetEntries()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(entries))

	for i, entry := range entries {
		names[i] = entry.Name
	}

	return names, nil
}

// GetEntries returns the entries as a list.
func (t *TarFile) GetEntries() ([]*tar.Header, error) {
	if t.loaded {
		return t.entries, nil
	}

	reader, err := newTarReader(t)
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	for {
		header, err := reader.Next()

		if err == io.EOF {
			return t.entries, nil
		}

		if err != nil {
			return nil, err
		}

		t.entries = append(t.entries, header)
	}
}

// Extract extracts one specific path into a directory.
// Parameter `name` is the archive path to be extracted.
// To extract all files `name` must be empty or "."
func (t *TarFile) Extract(name, targetDir string) error {
	reader, err := newTarReader(t)
	if err != nil {
		return err
	}

	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		relativeName, err := filepath.Rel(name, header.Name)
		if err != nil || strings.HasPrefix(relativeName, "..") {
			continue
		}

		filename := path.Join(targetDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			// maybe 0755 ???
			if err = os.MkdirAll(filename, os.FileMode(header.Mode)); err != nil {
				return nil
			}
		case tar.TypeReg:
			if err = os.MkdirAll(path.Dir(filename), 0755); err != nil {
				return err
			}

			file, err := os.Create(filename)
			if err != nil {
				return err
			}

			defer file.Close()

			if _, err = io.Copy(file, t.reader); err != nil {
				return err
			}

			if err = os.Chmod(filename, os.FileMode(header.Mode)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Not supported type : %c in file %s", header.Typeflag, filename)
		}
	}
}

// Next advances to the next entry in the tar archive.
// io.EOF is returned at the end of the input.
func (t *TarFile) Next() (*tar.Header, error) {
	return t.reader.Next()
}

// Read reads one specific path and returns a buffered reader.
// If path is not found it returns `ErrPathNotFound`
// If path is not a regular file it returns `nil`
func (t *TarFile) Read(name string) (*tar.Header, *bufio.Reader, error) {
	reader, err := newTarReader(t)
	if err != nil {
		return nil, nil, err
	}

	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil, nil, ErrPathNotFound
		}
		if err != nil {
			return nil, nil, err
		}

		if name != header.Name {
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg:
			return header, bufio.NewReader(reader), nil
		default:
			return nil, nil, nil
		}
	}
}

// Write writes a new file into tar file.
func (t *TarFile) Write(header *tar.Header, reader io.Reader) error {
	if err := t.writer.WriteHeader(header); err != nil {
		return err
	}

	if reader == nil {
		return nil
	}

	_, err := io.Copy(t.writer, reader)
	return err
}
