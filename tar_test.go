package garchive

import "testing"

func TestAddFile(t *testing.T) {
	tarfile, err := NewTarFile("tests/addfile.tar.gz", Gzip)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Add("tests/folder/a.txt", nil)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Add("tests/folder/b.txt", nil)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Close()

	if err != nil {
		t.Fatal(err)
	}
}

func TestAddFolder(t *testing.T) {
	tarfile, err := NewTarFile("tests/addfolder.tar.gz", Gzip)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Add("tests/folder", nil)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Close()

	if err != nil {
		t.Fatal(err)
	}
}

func TestIncludeSourceDir(t *testing.T) {
	tarfile, err := NewTarFile("tests/includesourcedir.tar.gz", Gzip)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Add("tests/folder", &TarAddOptions{Recursive: true, IncludeSourceDir: true})

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Close()

	if err != nil {
		t.Fatal(err)
	}
}

func TestWithoutRecursivity(t *testing.T) {
	tarfile, err := NewTarFile("tests/without_recursivity.tar.gz", Gzip)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Add("tests/folder", &TarAddOptions{Recursive: false})

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Close()

	if err != nil {
		t.Fatal(err)
	}
}

func TestWithoutCompression(t *testing.T) {
	tarfile, err := NewTarFile("tests/without_compression.tar", Uncompressed)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Add("tests/folder", nil)

	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Close()

	if err != nil {
		t.Fatal(err)
	}
}
