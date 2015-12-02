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
)

// Compression is the state represtents if compressed or not.
type Compression int

const (
	// Uncompressed represents the uncompressed.
	Uncompressed Compression = iota
	// Gzip is gzip compression algorithm.
	Gzip
)

// TarOptions is the compression configuration
type TarOptions struct {
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

type tarFile struct {
	Name           string
	File           *os.File
	TarReader      *tar.Reader
	TarWriter      *tar.Writer
	CompressReader io.ReadCloser
	CompressWriter io.WriteCloser
}

// Tar compress a source path into a tar file.
// It supports compressed and uncompressed format
func Tar(name, srcPath string, options *TarOptions) (err error) {
	if options == nil {
		options = &TarOptions{}
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// We create a tar file on disk
	tarFile, err := createTarFile(name, options)
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

// UnTar extracts the files from a tar file into a target directory
func UnTar(name, targetDir string, options *UnTarOptions) error {
	if options == nil {
		options = &UnTarOptions{}
	}

	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return err
	}

	tarFile, err := openTarFile(name)
	if err != nil {
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

		if err := extractTarFile(filePath, header, tarFile.TarReader); err != nil {
			return err
		}
	}
}

func createTarFile(name string, options *TarOptions) (*tarFile, error) {
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
	file, err := os.OpenFile(name, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}

	compression, err := detectCompression(file)
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

func extractTarFile(filePath string, header *tar.Header, reader *tar.Reader) error {
	// header.Mode is in linux format, we have to converto os.FileMode,
	// to be compatible to windows, ...
	headerInfo := header.FileInfo()

	switch header.Typeflag {
	case tar.TypeDir:
		fileInfo, err := os.Lstat(filePath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		// When the `filePath` already exists on disk and it is a regular file
		// it must be deleted in order to create the directory otherwise we should return an error.
		// When `filePath` already exists on dis and it is a directory
		// we try to delete it in order to create the extracted directory,
		// if it is not possible we are going to use this directory to extract the files.

		if err == nil {
			if err := os.Remove(filePath); err != nil && !fileInfo.IsDir() {
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
