package garchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
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
	reader         *tar.Reader
	compressWriter io.WriteCloser
	compressReader io.ReadCloser
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

	compression, err := detectCompression(file)
	if err != nil {
		return nil, err
	}

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

// ExtractAll ...
func (t *TarFile) ExtractAll(targetDir string) error {
	t.ensureReader()

	for {
		header, err := t.reader.Next()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		filename := path.Join(targetDir, header.Name)

		fmt.Println(filename)

		switch header.Typeflag {
		case tar.TypeDir:
			// maybe 0755 ???
			if err = os.MkdirAll(filename, os.FileMode(header.Mode)); err != nil {
				return nil
			}
		case tar.TypeReg:
			fmt.Println(header.Mode)
			if err = os.MkdirAll(path.Dir(filename), os.FileMode(header.Mode)); err != nil {
				return err
			}

			file, err := os.Create(filename)
			if err != nil {
				return err
			}

			if _, err = io.Copy(file, t.reader); err != nil {
				return err
			}

			if err = os.Chmod(filename, os.FileMode(header.Mode)); err != nil {
				return err
			}

			file.Close()
		default:
			return fmt.Errorf("Not supported ype : %c in file %s", header.Typeflag, filename)
		}
	}
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

func (t *TarFile) ensureReader() (err error) {
	if t.reader != nil {
		return
	}

	if t.Compression == Gzip {
		if t.compressReader, err = gzip.NewReader(t.file); err != nil {
			return
		}
	}

	if t.compressReader == nil {
		t.reader = tar.NewReader(t.file)
	} else {
		t.reader = tar.NewReader(t.compressReader)
	}

	return
}

func detectCompression(file *os.File) (Compression, error) {
	source := make([]byte, 4)

	currentPost, err := file.Seek(0, os.SEEK_CUR)
	if err != nil {
		return Uncompressed, err
	}

	if _, err := file.Read(source); err != nil {
		return Uncompressed, err
	}

	if _, err = file.Seek(currentPost, os.SEEK_SET); err != nil {
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
