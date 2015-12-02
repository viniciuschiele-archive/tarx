package archive

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTarFile(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(headers))
	assert.Equal(t, "a.txt", headers[0].Name)
}

func TestTarFolder(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 7, len(headers))
	assert.Equal(t, "a.txt", headers[0].Name)
	assert.Equal(t, "b.txt", headers[1].Name)
	assert.Equal(t, "c", headers[2].Name)
	assert.Equal(t, "c/c1.txt", headers[3].Name)
	assert.Equal(t, "c/c2.txt", headers[4].Name)
	assert.Equal(t, "d", headers[5].Name)
	assert.Equal(t, "symlink.txt", headers[6].Name)
}

func TestTarFolderWithIncludeSourceDir(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", &TarOptions{IncludeSourceDir: true})
	assert.NoError(t, err)
	defer os.Remove(filename)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 8, len(headers))
	assert.Equal(t, "input", headers[0].Name)
	assert.Equal(t, "input/a.txt", headers[1].Name)
	assert.Equal(t, "input/b.txt", headers[2].Name)
	assert.Equal(t, "input/c", headers[3].Name)
	assert.Equal(t, "input/c/c1.txt", headers[4].Name)
	assert.Equal(t, "input/c/c2.txt", headers[5].Name)
	assert.Equal(t, "input/d", headers[6].Name)
	assert.Equal(t, "input/symlink.txt", headers[7].Name)
}

func TestAppendCompressedTar(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input/c", &TarOptions{Compression: Gzip})
	assert.NoError(t, err)
	defer os.Remove(filename)

	err = Tar(filename, "tests/input/a.txt", &TarOptions{Append: true})
	assert.EqualError(t, ErrAppendNotSupported, err.Error())
}

func TestReadTar(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	header, reader, err := ReadTar(filename, "a.txt")
	assert.Equal(t, nil, err)
	assert.Equal(t, "a.txt", header.Name)
	b, _ := ioutil.ReadAll(reader)
	assert.Equal(t, "a.txt\n", string(b))
	assert.Equal(t, nil, reader.Close())
}

func TestReadTarDir(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	_, reader, err := ReadTar(filename, "c")
	assert.Equal(t, nil, reader)
	assert.Equal(t, nil, err)
}

func TestReadTarNotExist(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	_, _, err = ReadTar(filename, "notExists.txt")
	assert.Equal(t, os.ErrNotExist, err)
}

func TestUnTar(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	err = UnTar(filename, "tests/output", nil)
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, true, pathExists("tests/output/b.txt"))
	assert.Equal(t, true, pathExists("tests/output/symlink.txt"))
	assert.Equal(t, true, pathExists("tests/output/c"))
	assert.Equal(t, true, pathExists("tests/output/c/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c/c2.txt"))
	assert.Equal(t, true, pathExists("tests/output/d"))
}

func TestUnTarWithFlatDir(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	err = UnTar(filename, "tests/output", &UnTarOptions{FlatDir: true})
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, true, pathExists("tests/output/b.txt"))
	assert.Equal(t, true, pathExists("tests/output/symlink.txt"))
	assert.Equal(t, false, pathExists("tests/output/c"))
	assert.Equal(t, true, pathExists("tests/output/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c2.txt"))
	assert.Equal(t, false, pathExists("tests/output/d"))
}

func TestUnTarWithFilters(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	filters := []string{"a.txt", "c/c2.txt", "d"}
	err = UnTar(filename, "tests/output", &UnTarOptions{Filters: filters})
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, false, pathExists("tests/output/b.txt"))
	assert.Equal(t, false, pathExists("tests/output/symlink.txt"))
	assert.Equal(t, true, pathExists("tests/output/c"))
	assert.Equal(t, false, pathExists("tests/output/c/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c/c2.txt"))
	assert.Equal(t, true, pathExists("tests/output/d"))
}

func TestUnTarWithOverride(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	os.MkdirAll("tests/output/c", os.ModePerm)
	writeContent("tests/output/a.txt", "new a.txt")
	writeContent("tests/output/c/z.txt", "z.txt")

	err = UnTar(filename, "tests/output", nil)
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, true, pathExists("tests/output/b.txt"))
	assert.Equal(t, true, pathExists("tests/output/c"))
	assert.Equal(t, true, pathExists("tests/output/c/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c/c2.txt"))
	assert.Equal(t, true, pathExists("tests/output/c/z.txt"))
	assert.Equal(t, true, pathExists("tests/output/d"))

	assert.Equal(t, "a.txt\n", readContent("tests/output/a.txt"))
	assert.Equal(t, "z.txt", readContent("tests/output/c/z.txt"))
}

func TestUnTarWithoutOverride(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	os.MkdirAll("tests/output", os.ModePerm)
	writeContent("tests/output/a.txt", "new a.txt")

	err = UnTar(filename, "tests/output", &UnTarOptions{NoOverride: true})
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, "new a.txt", readContent("tests/output/a.txt"))
}

func TestAppendTar(t *testing.T) {
	filename := "tests/test.tar"

	err := Tar(filename, "tests/input/c", nil)
	assert.NoError(t, err)
	defer os.Remove(filename)

	err = Tar(filename, "tests/input/a.txt", &TarOptions{Append: true})
	assert.NoError(t, err)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 3, len(headers))
	assert.Equal(t, "c1.txt", headers[0].Name)
	assert.Equal(t, "c2.txt", headers[1].Name)
	assert.Equal(t, "a.txt", headers[2].Name)
}

func pathExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		return false
	}
	return true
}

func readContent(filePath string) string {
	file, _ := os.OpenFile(filePath, os.O_RDWR, os.ModePerm)
	defer file.Close()
	content, _ := ioutil.ReadAll(file)
	return string(content)
}

func writeContent(filePath, content string) {
	file, _ := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	defer file.Close()
	file.WriteString(content)
}
