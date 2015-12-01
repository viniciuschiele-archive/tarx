package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

// TarOptions ...
type TarOptions struct {
	Compression      Compression
	IncludeSourceDir bool
}

// UnTarOptions ...
type UnTarOptions struct {
	FlatDir bool
	Filters []string
}

type tarFile struct {
	Name           string
	File           *os.File
	TarReader      *tar.Reader
	TarWriter      *tar.Writer
	CompressReader io.ReadCloser
	CompressWriter io.WriteCloser
}

// Tar ...
func Tar(name, srcPath string, options *TarOptions) error {
	if options == nil {
		options = &TarOptions{}
	}

	tarFile, err := newTarFile(name, options)
	if err != nil {
		return err
	}

	defer tarFile.Close()

	fileInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// Removes the last slash to avoid different behaviors when `name` is a folder
	srcPath = path.Clean(srcPath)
	baseDir := path.Dir(srcPath)

	if fileInfo.IsDir() && !options.IncludeSourceDir {
		baseDir = srcPath
	}

	return filepath.Walk(srcPath,
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

			if err := tarFile.TarWriter.WriteHeader(header); err != nil {
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

			_, err = io.Copy(tarFile.TarWriter, file)
			return err
		})
}

// ListTar ...
func ListTar(name string) ([]*tar.Header, error) {
	tarFile, err := openTarFile(name)
	if err != nil {
		return nil, err
	}

	headers := []*tar.Header{}

	for {
		header, err := tarFile.TarReader.Next()
		if err == io.EOF {
			return headers, nil
		}
		if err != nil {
			return nil, err
		}

		headers = append(headers, header)
	}
}

// UnTar ...
func UnTar(name, targetDir string, options *UnTarOptions) error {
	if options == nil {
		options = &UnTarOptions{}
	}

	tarFile, err := openTarFile(name)
	if err != nil {
		return err
	}

	for {
		header, err := tarFile.TarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		filename := header.Name

		if options.Filters != nil {
			matched := false
			for _, filter := range options.Filters {
				if strings.HasSuffix(filename, filter) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		if options.FlatDir {
			if header.Typeflag == tar.TypeDir {
				continue
			}
			filename = filepath.Base(filename)
		}

		if !path.IsAbs(filename) {
			filename = path.Join(targetDir, filename)
		}

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

			if _, err = io.Copy(file, tarFile.TarReader); err != nil {
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

func newTarFile(name string, options *TarOptions) (*tarFile, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	var tarWriter *tar.Writer
	var compressWriter io.WriteCloser

	if options.Compression == Gzip {
		compressWriter = gzip.NewWriter(file)
	}

	if compressWriter == nil {
		tarWriter = tar.NewWriter(file)
	} else {
		tarWriter = tar.NewWriter(compressWriter)
	}

	return &tarFile{
		Name:           name,
		File:           file,
		TarWriter:      tarWriter,
		CompressWriter: compressWriter,
	}, nil
}

func openTarFile(name string) (*tarFile, error) {
	file, err := os.OpenFile(name, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	compression, err := getTarCompression(file)
	if err != nil {
		return nil, err
	}

	var tarReader *tar.Reader
	var compressReader io.ReadCloser

	if compression == Gzip {
		if compressReader, err = gzip.NewReader(file); err != nil {
			return nil, err
		}
	}

	if compressReader == nil {
		tarReader = tar.NewReader(file)
	} else {
		tarReader = tar.NewReader(compressReader)
	}

	return &tarFile{
		Name:           name,
		File:           file,
		TarReader:      tarReader,
		CompressReader: compressReader,
	}, nil
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

func (t *tarFile) Close() error {
	if t.TarWriter != nil {
		if err := t.TarWriter.Close(); err != nil {
			return err
		}
	}

	if t.CompressReader != nil {
		return t.CompressReader.Close()
	}

	if t.CompressWriter != nil {
		return t.CompressWriter.Close()
	}

	return t.File.Close()
}
