package archive

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZipFile(t *testing.T) {
	filename := "tests/test.zip"

	err := Zip(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	headers, err := ListZip(filename)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(headers))
	assert.Equal(t, "a.txt", headers[0].Name)
}

func TestZipFolder(t *testing.T) {
	filename := "tests/test.zip"

	err := Zip(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	headers, err := ListZip(filename)
	assert.NoError(t, err)

	assert.Equal(t, 7, len(headers))
	assert.Equal(t, "a.txt", headers[0].Name)
	assert.Equal(t, "b.txt", headers[1].Name)
	assert.Equal(t, "c/", headers[2].Name)
	assert.Equal(t, "c/c1.txt", headers[3].Name)
	assert.Equal(t, "c/c2.txt", headers[4].Name)
	assert.Equal(t, "d/", headers[5].Name)
	assert.Equal(t, "symlink.txt", headers[6].Name)
}

func TestZipFolderWithIncludeSourceDir(t *testing.T) {
	filename := "tests/test.zip"

	err := Zip(filename, "tests/input", &ZipOptions{IncludeSourceDir: true})
	assert.NoError(t, err)
	defer os.Remove(filename)

	headers, err := ListZip(filename)
	assert.NoError(t, err)

	assert.Equal(t, 8, len(headers))
	assert.Equal(t, "input/", headers[0].Name)
	assert.Equal(t, "input/a.txt", headers[1].Name)
	assert.Equal(t, "input/b.txt", headers[2].Name)
	assert.Equal(t, "input/c/", headers[3].Name)
	assert.Equal(t, "input/c/c1.txt", headers[4].Name)
	assert.Equal(t, "input/c/c2.txt", headers[5].Name)
	assert.Equal(t, "input/d/", headers[6].Name)
	assert.Equal(t, "input/symlink.txt", headers[7].Name)
}

func TestReadZip(t *testing.T) {
	filename := "tests/test.zip"

	err := Zip(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	file, err := ReadZip(filename, "a.txt")

	assert.Equal(t, nil, err)
	assert.Equal(t, "a.txt", header.Name)
	b, _ := ioutil.ReadAll(reader)
	assert.Equal(t, "a.txt\n", string(b))
	assert.Equal(t, nil, reader.Close())
}
