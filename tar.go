package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
	// ErrAppendNotSupported means append cannot be used on compressed files
	ErrAppendNotSupported = errors.New("Append is only supported on compressed files")
)

// TarOptions is the compression configuration
type TarOptions struct {
	Append           bool
	Compression      Compression
	IncludeSourceDir bool
	Filters          []string
}

// UnTarOptions is the decompression configuration
type UnTarOptions struct {
	FlatDir    bool
	Filters    []string
	NoOverride bool
}

// tarFile holds all resources for the opened tar file
type tarFile struct {
	Name           string
	File           *os.File
	TarReader      *tar.Reader
	TarWriter      *tar.Writer
	CompressReader io.ReadCloser
	CompressWriter io.WriteCloser
}

// tarReader is used to expose the tar file to the user
// Close needs to call in order to close the tar file.
type tarReader struct {
	io.ReadCloser
	TarFile *tarFile
}

// Tar compress a source path into a tar file.
// It supports compressed and uncompressed format
func Tar(name, srcPath string, options *TarOptions) (err error) {
	if options == nil {
		options = &TarOptions{}
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return
	}

	var tarFile *tarFile

	if options.Append {
		tarFile, err = openTarFile(name, true)
	} else {
		tarFile, err = createTarFile(name, options.Compression)
	}

	if err != nil {
		return
	}

	// If any error occurs we delete the tar file
	defer func() {
		closeTarFile(tarFile, err != nil)
	}()

	// Removes the last slash to avoid different behaviors when `srcPath` is a folder
	srcPath = path.Clean(srcPath)

	// All files added are relative to the tar file
	// If IncludeSourceDir is true one level behind is added
	relPath := path.Dir(srcPath)
	if srcInfo.IsDir() && !options.IncludeSourceDir {
		relPath = srcPath
	}

	// To improve performance filters are prepared before.
	filters := prepareFilters(options.Filters)

	err = filepath.Walk(srcPath,
		func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Makes the file to be relative to the tar file
			// We don't support absolute path while compressing
			// but it can be done further
			relFilePath, err := filepath.Rel(relPath, filePath)
			if err != nil {
				return err
			}

			// When IncludeSourceDir is false the relative path for the
			// root folder is '.', we have to ignore this folder
			if relFilePath == "." {
				return nil
			}

			// Check if we have to add the current file based on the user filters
			if !optimizedMatches(relFilePath, filters) {
				return nil
			}

			// All good, relative path made, filters applied, now we can write
			// the user file into tar file
			return writeTarFile(filePath, relFilePath, tarFile.TarWriter)
		})

	return
}

// ListTar lists all entries from a tar file
func ListTar(name string) ([]*tar.Header, error) {
	tarFile, err := openTarFile(name, false)
	if err != nil {
		return nil, err
	}

	defer closeTarFile(tarFile, false)

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

// ReadTar reads a specific file from the tar file.
// If the file is not a regular file it returns a reader nil
func ReadTar(name string, fileName string) (*tar.Header, io.ReadCloser, error) {
	tarFile, err := openTarFile(name, false)
	if err != nil {
		return nil, nil, err
	}

	name = path.Clean(fileName)

	for {
		header, err := tarFile.TarReader.Next()
		if err == io.EOF {
			closeTarFile(tarFile, false)
			return nil, nil, os.ErrNotExist
		}
		if err != nil {
			closeTarFile(tarFile, false)
			return nil, nil, err
		}

		// If the file found is not a regular file we don't return a reader
		if name == header.Name {
			if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
				return header, &tarReader{TarFile: tarFile}, nil
			}
			closeTarFile(tarFile, false)
			return header, nil, nil
		}
	}
}

// UnTar extracts the files from a tar file into a target directory
func UnTar(name, targetDir string, options *UnTarOptions) error {
	if options == nil {
		options = &UnTarOptions{}
	}

	tarFile, err := openTarFile(name, false)
	if err != nil {
		return err
	}

	defer closeTarFile(tarFile, false)

	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return err
	}

	// To improve performance the filters are prepared before.
	filters := prepareFilters(options.Filters)

	for {
		header, err := tarFile.TarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Removes the last slash to avoid different behaviors when `header.Name` is a folder
		filePath := filepath.Clean(header.Name)

		// Check if we have to extact the current file based on the user filters
		if !optimizedMatches(filePath, filters) {
			continue
		}

		// If FlatDir is true we have to extract all files into root folder
		// and we have to ignore all sub directories
		if options.FlatDir {
			if header.Typeflag == tar.TypeDir {
				continue
			}
			filePath = filepath.Base(filePath)
		}

		// If `filePath` is an absolute path we are going to extract it
		// relative to the `targetDir`
		filePath = path.Join(targetDir, filePath)

		if err := extractTarFile(filePath, header, tarFile.TarReader, options.NoOverride); err != nil {
			return err
		}
	}
}

func createTarFile(name string, compression Compression) (*tarFile, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	var tarWriter *tar.Writer
	var compressWriter io.WriteCloser

	if compression == Gzip {
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

func openTarFile(name string, append bool) (*tarFile, error) {
	file, err := os.OpenFile(name, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	// Reads the header from the file to see which compression
	// this file has been using.
	compression, err := detectCompression(file)
	if err != nil {
		return nil, err
	}

	// I have found only this hack to append files into tar file.
	// It works only for uncompressed tar files :(
	// http://stackoverflow.com/questions/18323995/golang-append-file-to-an-existing-tar-archive
	// We may improve it in the future.
	if append {
		if compression != Uncompressed {
			return nil, ErrAppendNotSupported
		}

		if _, err = file.Seek(-2<<9, os.SEEK_END); err != nil {
			return nil, err
		}
	}

	var tarReader *tar.Reader
	var tarWriter *tar.Writer
	var compressReader io.ReadCloser

	if append {
		tarWriter = tar.NewWriter(file)
	}

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
		TarWriter:      tarWriter,
		CompressReader: compressReader,
	}, nil
}

func extractTarFile(filePath string, header *tar.Header, reader *tar.Reader, noOverride bool) error {
	// header.Mode is in linux format, we have to converto os.FileMode,
	// to be compatible to windows, ...
	headerInfo := header.FileInfo()

	switch header.Typeflag {
	case tar.TypeDir:
		fileInfo, err := os.Lstat(filePath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// If the `filePath` already exists on disk and it is a regular file
		// it must be deleted in order to create the directory otherwise we should return an error.

		if err == nil && !fileInfo.IsDir() {
			if err := os.Remove(filePath); err != nil {
				return err
			}
		}

		if err := os.Mkdir(filePath, headerInfo.Mode()); err != nil && !os.IsExist(err) {
			return err
		}

		return nil
	case tar.TypeReg, tar.TypeRegA:
		_, err := os.Lstat(filePath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// When the `filePath` already exists on disk it must be deleted
		// in order to create the extracted file
		if err == nil {
			if noOverride {
				return nil
			}

			if err := os.Remove(filePath); err != nil {
				return err
			}
		}

		if err := createFile(filePath, headerInfo.Mode(), reader); err != nil {
			return err
		}

		return nil
	default:
		return fmt.Errorf("File type not supported: %c", header.Typeflag)
	}
}

func writeTarFile(filePath, name string, writer *tar.Writer) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return err
	}

	header.Name = name

	if err := writer.WriteHeader(header); err != nil {
		return err
	}

	if header.Typeflag != tar.TypeReg {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(writer, file)
	return err
}

func closeTarFile(tf *tarFile, remove bool) error {
	if tf.TarWriter != nil {
		if err := tf.TarWriter.Close(); err != nil {
			return err
		}
	}

	if tf.CompressReader != nil {
		return tf.CompressReader.Close()
	}

	if tf.CompressWriter != nil {
		return tf.CompressWriter.Close()
	}

	if err := tf.File.Close(); err != nil {
		return err
	}

	if remove {
		return os.Remove(tf.Name)
	}

	return nil
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

func (r *tarReader) Read(p []byte) (n int, err error) {
	return r.TarFile.TarReader.Read(p)
}

func (r *tarReader) Close() error {
	return closeTarFile(r.TarFile, false)
}
