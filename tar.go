package garchive

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
	// ErrTarTypeNotSupported returned when a header with tar type is not supported.
	ErrTarTypeNotSupported = errors.New("Tar type not supported.")
)

// TarFile represents a tar file in disk.
// It can be compressed (gzip) or not.
type TarFile struct {
	Name           string
	Compression    Compression
	file           *os.File
	tarWriter      *tar.Writer
	tarReader      *tar.Reader
	compressWriter io.WriteCloser
	compressReader io.ReadCloser
	startPosition  int64
}

// TarAddOptions ...
type TarAddOptions struct {
	IncludeSourceDir bool // Only used when the path is a directory
}

// NewTarFile creates a new tar file on disk.
func NewTarFile(name string, compression Compression) (*TarFile, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	tarfile := &TarFile{Name: name, Compression: compression, file: file}
	tarfile.initWriter()
	return tarfile, nil
}

// OpenTarFile opens a tar file on disk.
func OpenTarFile(name string) (*TarFile, error) {
	file, err := os.OpenFile(name, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	compression, err := detectCompression(file)
	if err != nil {
		return nil, err
	}

	tarfile := &TarFile{Name: name, Compression: compression, file: file}
	tarfile.initReader()
	return tarfile, nil
}

// Close closes the tar file., flushing any unwritten
// data to the underlying writer.
func (t *TarFile) Close() error {
	if t.tarWriter != nil {
		if err := t.tarWriter.Close(); err != nil {
			return err
		}
	}

	if t.compressReader != nil {
		if err := t.compressReader.Close(); err != nil {
			return err
		}
	}

	if t.compressWriter != nil {
		if err := t.compressWriter.Close(); err != nil {
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

			if err := t.tarWriter.WriteHeader(header); err != nil {
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

			_, err = io.Copy(t.tarWriter, file)
			return err
		})
}

// Extract extracts one specific path into a directory.
// Parameter `name` is the archive path to be extracted.
// To extract all files `name` must be empty or "."
func (t *TarFile) Extract(name, targetDir string) error {
	if t.tarReader == nil {
		return errors.New("File was not opened for reading")
	}

	_, err := t.file.Seek(t.startPosition, os.SEEK_SET)
	if err != nil {
		return err
	}

	for {
		header, err := t.tarReader.Next()
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

			if _, err = io.Copy(file, t.tarReader); err != nil {
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

// Read reads one specific path and returns a buffered reader.
// If path is not found it returns `ErrNotFound`
// If path is not a regular file or directory it returns `ErrTarTypeNotSupported`
func (t *TarFile) Read(name string) (*bufio.Reader, error) {
	if t.tarReader == nil {
		return nil, errors.New("File was not opened for reading")
	}

	_, err := t.file.Seek(t.startPosition, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	for {
		header, err := t.tarReader.Next()
		if err == io.EOF {
			return nil, ErrPathNotFound
		}

		if err != nil {
			return nil, err
		}

		if name != header.Name {
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg:
			buffer := new(bytes.Buffer)
			writer := bufio.NewWriter(buffer)

			if _, err := io.Copy(writer, t.tarReader); err != nil {
				return nil, err
			}

			err := writer.Flush()
			if err != nil {
				return nil, err
			}

			return bufio.NewReader(buffer), nil
		default:
			return nil, ErrTarTypeNotSupported
		}
	}
}

func (t *TarFile) initReader() (err error) {
	if t.tarReader != nil {
		return
	}

	if t.Compression == Gzip {
		if t.compressReader, err = gzip.NewReader(t.file); err != nil {
			return
		}
	}

	if t.compressReader == nil {
		t.tarReader = tar.NewReader(t.file)
	} else {
		t.tarReader = tar.NewReader(t.compressReader)
	}

	t.startPosition, err = t.file.Seek(0, os.SEEK_CUR)

	return
}

func (t *TarFile) initWriter() {
	if t.tarWriter != nil {
		return
	}

	if t.Compression == Gzip {
		t.compressWriter = gzip.NewWriter(t.file)
	}

	if t.compressWriter == nil {
		t.tarWriter = tar.NewWriter(t.file)
	} else {
		t.tarWriter = tar.NewWriter(t.compressWriter)
	}
}

func detectCompression(file *os.File) (Compression, error) {
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
