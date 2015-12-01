package archive

import (
	"archive/tar"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddFile(t *testing.T) {
	filename := "tests/test.tar"

	tarfile, err := NewTarFile(filename, Uncompressed)
	assert.NoError(t, err)

	defer func() {
		tarfile.Close()
		os.Remove(filename)
	}()

	err = tarfile.Add("tests/folder/a.txt", nil)
	assert.NoError(t, err)

	err = tarfile.Add("tests/folder/b.txt", nil)
	assert.NoError(t, err)

	err = tarfile.Close()
	assert.NoError(t, err)

	tarfile, err = OpenTarFile(filename)
	assert.NoError(t, err)

	names, err := tarfile.GetNames()
	assert.NoError(t, err)

	assert.Equal(t, 2, len(names))
	assert.Equal(t, "a.txt", names[0])
	assert.Equal(t, "b.txt", names[1])
}

func TestAddFolder(t *testing.T) {
	filename := "tests/test.tar"

	tarfile, err := NewTarFile(filename, Uncompressed)
	assert.NoError(t, err)

	defer func() {
		tarfile.Close()
		os.Remove(filename)
	}()

	err = tarfile.Add("tests/folder", nil)
	assert.NoError(t, err)

	err = tarfile.Close()
	assert.NoError(t, err)

	tarfile, err = OpenTarFile(filename)
	assert.NoError(t, err)

	names, err := tarfile.GetNames()
	assert.NoError(t, err)

	assert.Equal(t, 6, len(names))
	assert.Equal(t, "a.txt", names[0])
	assert.Equal(t, "b.txt", names[1])
	assert.Equal(t, "c", names[2])
	assert.Equal(t, "c/c1.txt", names[3])
	assert.Equal(t, "c/c2.txt", names[4])
	assert.Equal(t, "d", names[5])
}

func TestAddFolderWithIncludeSourceFolder(t *testing.T) {
	filename := "tests/test.tar"

	tarfile, err := NewTarFile(filename, Uncompressed)
	assert.NoError(t, err)

	defer func() {
		tarfile.Close()
		os.Remove(filename)
	}()

	err = tarfile.Add("tests/folder", &TarAddOptions{IncludeSourceDir: true})
	assert.NoError(t, err)

	err = tarfile.Close()
	assert.NoError(t, err)

	tarfile, err = OpenTarFile(filename)
	assert.NoError(t, err)

	names, err := tarfile.GetNames()
	assert.NoError(t, err)

	assert.Equal(t, 7, len(names))
	assert.Equal(t, "folder", names[0])
	assert.Equal(t, "folder/a.txt", names[1])
	assert.Equal(t, "folder/b.txt", names[2])
	assert.Equal(t, "folder/c", names[3])
	assert.Equal(t, "folder/c/c1.txt", names[4])
	assert.Equal(t, "folder/c/c2.txt", names[5])
	assert.Equal(t, "folder/d", names[6])
}

func TestGetEntries(t *testing.T) {
	filename := "tests/test.tar"

	tarfile, err := NewTarFile(filename, Uncompressed)
	assert.NoError(t, err)

	defer func() {
		tarfile.Close()
		os.Remove(filename)
	}()

	err = tarfile.Add("tests/folder", nil)
	assert.NoError(t, err)

	err = tarfile.Close()
	assert.NoError(t, err)

	tarfile, err = OpenTarFile(filename)
	assert.NoError(t, err)

	entries, err := tarfile.GetEntries()
	assert.NoError(t, err)

	entry := entries[0]
	assert.Equal(t, "a.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry = entries[1]
	assert.Equal(t, "b.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry = entries[2]
	assert.Equal(t, "c", entry.Name)
	assert.Equal(t, string(tar.TypeDir), string(entry.Typeflag))

	entry = entries[3]
	assert.Equal(t, "c/c1.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry = entries[4]
	assert.Equal(t, "c/c2.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry = entries[5]
	assert.Equal(t, "d", entry.Name)
	assert.Equal(t, string(tar.TypeDir), string(entry.Typeflag))
}

func TestNext(t *testing.T) {
	filename := "tests/test.tar"

	tarfile, err := NewTarFile(filename, Uncompressed)
	assert.NoError(t, err)

	defer func() {
		tarfile.Close()
		os.Remove(filename)
	}()

	err = tarfile.Add("tests/folder", nil)
	assert.NoError(t, err)

	err = tarfile.Close()
	assert.NoError(t, err)

	tarfile, err = OpenTarFile(filename)
	assert.NoError(t, err)

	entry, err := tarfile.Next()
	assert.NoError(t, err)
	assert.Equal(t, "a.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry, err = tarfile.Next()
	assert.NoError(t, err)
	assert.Equal(t, "b.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry, err = tarfile.Next()
	assert.NoError(t, err)
	assert.Equal(t, "c", entry.Name)
	assert.Equal(t, string(tar.TypeDir), string(entry.Typeflag))

	entry, err = tarfile.Next()
	assert.NoError(t, err)
	assert.Equal(t, "c/c1.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry, err = tarfile.Next()
	assert.NoError(t, err)
	assert.Equal(t, "c/c2.txt", entry.Name)
	assert.Equal(t, string(tar.TypeReg), string(entry.Typeflag))

	entry, err = tarfile.Next()
	assert.NoError(t, err)
	assert.Equal(t, "d", entry.Name)
	assert.Equal(t, string(tar.TypeDir), string(entry.Typeflag))
}

func TestRead(t *testing.T) {
	filename := "tests/test.tar"

	tarfile, err := NewTarFile(filename, Uncompressed)
	assert.NoError(t, err)

	defer func() {
		tarfile.Close()
		os.Remove(filename)
	}()

	err = tarfile.Add("tests/folder/a.txt", nil)
	assert.NoError(t, err)

	err = tarfile.Close()
	assert.NoError(t, err)

	tarfile, err = OpenTarFile(filename)
	assert.NoError(t, err)

	entry, reader, err := tarfile.Read("a.txt")
	assert.NoError(t, err)

	content := make([]byte, entry.Size)
	_, err = reader.Read(content)
	assert.NoError(t, err)

	assert.Equal(t, "a.txt", entry.Name)
	assert.Equal(t, "a.txt", string(content))
}
