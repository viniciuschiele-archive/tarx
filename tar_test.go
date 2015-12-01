package archive

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTarFile(t *testing.T) {
	filename := "tests/test.tar"
	defer os.Remove(filename)

	err := Tar(filename, "tests/input/a.txt", nil)
	assert.NoError(t, err)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(headers))
	assert.Equal(t, "a.txt", headers[0].Name)
}

func TestTarFolder(t *testing.T) {
	filename := "tests/test.tar"
	defer os.Remove(filename)

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 6, len(headers))
	assert.Equal(t, "a.txt", headers[0].Name)
	assert.Equal(t, "b.txt", headers[1].Name)
	assert.Equal(t, "c", headers[2].Name)
	assert.Equal(t, "c/c1.txt", headers[3].Name)
	assert.Equal(t, "c/c2.txt", headers[4].Name)
	assert.Equal(t, "d", headers[5].Name)
}

func TestTarFolderWithIncludeSourceFolder(t *testing.T) {
	filename := "tests/test.tar"
	defer os.Remove(filename)

	err := Tar(filename, "tests/input", &TarOptions{IncludeSourceDir: true})
	assert.NoError(t, err)

	headers, err := ListTar(filename)
	assert.NoError(t, err)

	assert.Equal(t, 7, len(headers))
	assert.Equal(t, "input", headers[0].Name)
	assert.Equal(t, "input/a.txt", headers[1].Name)
	assert.Equal(t, "input/b.txt", headers[2].Name)
	assert.Equal(t, "input/c", headers[3].Name)
	assert.Equal(t, "input/c/c1.txt", headers[4].Name)
	assert.Equal(t, "input/c/c2.txt", headers[5].Name)
	assert.Equal(t, "input/d", headers[6].Name)
}

func TestUnTar(t *testing.T) {
	filename := "tests/test.tar"
	defer os.Remove(filename)

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)

	err = UnTar(filename, "tests/output", nil)
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, true, pathExists("tests/output/b.txt"))
	assert.Equal(t, true, pathExists("tests/output/c"))
	assert.Equal(t, true, pathExists("tests/output/c/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c/c2.txt"))
	assert.Equal(t, true, pathExists("tests/output/d"))
}

func TestUnTarWithFlatDir(t *testing.T) {
	filename := "tests/test.tar"
	defer os.Remove(filename)

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)

	err = UnTar(filename, "tests/output", &UnTarOptions{FlatDir: true})
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, true, pathExists("tests/output/b.txt"))
	assert.Equal(t, false, pathExists("tests/output/c"))
	assert.Equal(t, true, pathExists("tests/output/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c2.txt"))
	assert.Equal(t, false, pathExists("tests/output/d"))
}

func TestUnTarWithFilters(t *testing.T) {
	filename := "tests/test.tar"
	defer os.Remove(filename)

	err := Tar(filename, "tests/input", nil)
	assert.NoError(t, err)

	filters := []string{"a.txt", "c/c2.txt"}
	err = UnTar(filename, "tests/output", &UnTarOptions{Filters: filters})
	assert.NoError(t, err)
	defer os.RemoveAll("tests/output")

	assert.Equal(t, true, pathExists("tests/output/a.txt"))
	assert.Equal(t, false, pathExists("tests/output/b.txt"))
	assert.Equal(t, true, pathExists("tests/output/c"))
	assert.Equal(t, false, pathExists("tests/output/c/c1.txt"))
	assert.Equal(t, true, pathExists("tests/output/c/c2.txt"))
	assert.Equal(t, false, pathExists("tests/output/d"))
}

func pathExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		return false
	}
	return true
}
