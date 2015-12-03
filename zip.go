package archive

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"path/filepath"
)

// ZipOptions is the compression configuration
type ZipOptions struct {
	Append           bool
	IncludeSourceDir bool
	Filters          []string
}

// UnZipOptions is the decompression configuration
type UnZipOptions struct {
	FlatDir    bool
	Filters    []string
	NoOverride bool
}

// ZipReader is used to expose the zip file to the user
// Close needs to be called in order to close the zip file.
type ZipReader struct {
	io.ReadCloser
	zipReader  *zip.ReadCloser
	fileReader io.ReadCloser
}

// zipFile holds all resources for the opened zip file
type zipFile struct {
	Name      string
	ZipReader *zip.ReadCloser
	ZipWriter *zip.Writer
}

// Zip compress a source path into a zip file.
func Zip(name, srcPath string, options *ZipOptions) (err error) {
	if options == nil {
		options = &ZipOptions{}
	}

	srcInfo, err := os.Lstat(srcPath)
	if err != nil {
		return
	}

	zipFile, err := createZipFile(name)
	if err != nil {
		return
	}

	// If any error occurs we delete the tar file
	defer func() {
		closeZipFile(zipFile, err != nil)
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
			return writeZipFile(filePath, relFilePath, zipFile.ZipWriter)
		})

	return
}

// ListZip lists all entries from a zip file.
func ListZip(name string) ([]*zip.File, error) {
	zipFile, err := openZipFile(name)
	if err != nil {
		return nil, err
	}

	defer closeZipFile(zipFile, false)

	return zipFile.ZipReader.File, nil
}

// IterZip returns a reader to iterate through the zip file.
func IterZip(name string) (*zip.ReadCloser, error) {
	zipFile, err := openZipFile(name)
	if err != nil {
		return nil, err
	}

	return zipFile.ZipReader, nil
}

// ReadZip reads a specific file from the zip file.
// If the file is not a regular file it returns a reader nil
func ReadZip(name, fileName string) (*zip.File, io.ReadCloser, error) {
	reader, err := IterZip(name)
	if err != nil {
		return nil, nil, err
	}

	name = path.Clean(fileName)

	for _, file := range reader.File {
		// If the file found is not a regular file we don't return a reader
		if name != path.Clean(file.Name) {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, nil, err
		}

		f := file
		r := &ZipReader{zipReader: reader, fileReader: fileReader}

		return f, r, nil
	}

	return nil, nil, os.ErrNotExist
}

// UnZip extracts the files from a tar file into a target directory
func UnZip(name, targetDir string, options *UnZipOptions) error {
	if options == nil {
		options = &UnZipOptions{}
	}

	zipFile, err := openZipFile(name)
	if err != nil {
		return err
	}

	defer closeZipFile(zipFile, false)

	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return err
	}

	// To improve performance the filters are prepared before.
	filters := prepareFilters(options.Filters)

	for _, file := range zipFile.ZipReader.File {
		// Removes the last slash to avoid different behaviors when `header.Name` is a folder
		filePath := filepath.Clean(file.Name)

		// Check if we have to extact the current file based on the user filters
		if !optimizedMatches(filePath, filters) {
			continue
		}

		// If FlatDir is true we have to extract all files into root folder
		// and we have to ignore all sub directories
		if options.FlatDir {
			if file.Mode()&os.ModeDir == os.ModeDir {
				continue
			}
			filePath = filepath.Base(filePath)
		}

		// If `filePath` is an absolute path we are going to extract it
		// relative to the `targetDir`
		filePath = path.Join(targetDir, filePath)

		if err := extractZipFile(filePath, file, options.NoOverride); err != nil {
			return err
		}
	}

	return nil
}

func createZipFile(name string) (*zipFile, error) {
	file, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	return &zipFile{
		Name:      name,
		ZipWriter: zip.NewWriter(file),
	}, nil
}

func openZipFile(name string) (*zipFile, error) {
	reader, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}

	return &zipFile{
		Name:      name,
		ZipReader: reader,
	}, nil
}

func extractZipFile(filePath string, file *zip.File, noOverride bool) error {
	fileInfo, err := os.Lstat(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the `filePath` already exists on disk and it is a file
	// we try to delete it in order to create a new one unless
	// `noOverride` is set to true
	if err == nil && !fileInfo.IsDir() {
		if noOverride {
			return nil
		}

		if err := os.Remove(filePath); err != nil {
			return err
		}
	}

	// header.Mode is in linux format, we have to convert it to os.FileMode
	// to be compatible to other platforms.
	headerInfo := file.FileInfo()
	mode := headerInfo.Mode()

	if mode&os.ModeDir == os.ModeDir {
		if err := os.Mkdir(filePath, mode); err != nil && !os.IsExist(err) {
			return err
		}
	} else if mode&os.ModeSymlink != os.ModeSymlink {
		reader, err := file.Open()
		if err != nil {
			return err
		}

		defer reader.Close()

		if err := createFile(filePath, mode, reader); err != nil {
			return err
		}
	}

	return nil
}

func writeZipFile(filePath, name string, writer *zip.Writer) error {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(fileInfo)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		name += string(os.PathSeparator)
	}

	header.Name = name

	if !fileInfo.IsDir() {
		header.Method = zip.Deflate
	}

	entryWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(entryWriter, file)
	return err
}

func closeZipFile(zf *zipFile, remove bool) error {
	if zf.ZipReader != nil {
		if err := zf.ZipReader.Close(); err != nil {
			return err
		}
	}

	if zf.ZipWriter != nil {
		if err := zf.ZipWriter.Close(); err != nil {
			return err
		}
	}

	if remove {
		return os.Remove(zf.Name)
	}

	return nil
}

// Read reads from the current entry in the tar archive.
// It returns 0, io.EOF when it reaches the end of that entry,
// until Next is called to advance to the next entry.
func (r *ZipReader) Read(p []byte) (n int, err error) {
	return r.fileReader.Read(p)
}

// Close closes the reader
func (r *ZipReader) Close() error {
	return nil
}
