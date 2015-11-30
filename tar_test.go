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

	err = tarfile.Add("tests/folder", &TarAddOptions{IncludeSourceDir: true})

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

func TestExtract(t *testing.T) {
	tarfile, err := NewTarFile("tests/extract.tar.gz", Gzip)
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

	tarfile, err = OpenTarFile("tests/extract.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Extract("f", "tests/extract")
	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestExtractAll(t *testing.T) {
	tarfile, err := NewTarFile("tests/extractall.tar.gz", Gzip)
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

	tarfile, err = OpenTarFile("tests/extractall.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	err = tarfile.Extract(".", "tests/extractall")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	tarfile, err := NewTarFile("tests/read.tar.gz", Gzip)
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

	tarfile, err = OpenTarFile("tests/read.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	reader, err := tarfile.Read("f/c.txt")
	if err != nil {
		t.Fatal(err)
	}

	line, _, err := reader.ReadLine()
	if err != nil {
		t.Fatal(err)
	}

	if string(line) != "c.txt" {
		t.Fatal("Invalid content")
	}

	err = tarfile.Close()
	if err != nil {
		t.Fatal(err)
	}
}
