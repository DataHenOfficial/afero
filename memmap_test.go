package afero

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNormalizePath(t *testing.T) {
	type test struct {
		input    string
		expected string
	}

	data := []test{
		{".", FilePathSeparator},
		{"./", FilePathSeparator},
		{"..", FilePathSeparator},
		{"../", FilePathSeparator},
		{"./..", FilePathSeparator},
		{"./../", FilePathSeparator},
		{"tmp", "/tmp"}, // "tmp" and "/tmp" are equivalent in MemMapFS
		{"/tmp", "/tmp"},
	}

	for i, d := range data {
		cpath := normalizePath(d.input)
		if d.expected != cpath {
			t.Errorf("Test %d failed. Expected %q got %q", i, d.expected, cpath)
		}
	}
}

func TestPathErrors(t *testing.T) {
	path := filepath.Join(".", "some", "path")
	path2 := filepath.Join(".", "different")
	path3 := filepath.Join(".", "different", "long", "path")
	fs := NewMemMapFs()
	perm := os.FileMode(0755)

	// relevant functions:
	// func (m *MemMapFs) Chmod(name string, mode os.FileMode) error
	// func (m *MemMapFs) Chtimes(name string, atime time.Time, mtime time.Time) error
	// func (m *MemMapFs) Create(name string) (File, error)
	// func (m *MemMapFs) Mkdir(name string, perm os.FileMode) error
	// func (m *MemMapFs) MkdirAll(path string, perm os.FileMode) error
	// func (m *MemMapFs) Open(name string) (File, error)
	// func (m *MemMapFs) OpenFile(name string, flag int, perm os.FileMode) (File, error)
	// func (m *MemMapFs) Remove(name string) error
	// func (m *MemMapFs) Rename(oldname, newname string) error
	// func (m *MemMapFs) Stat(name string) (os.FileInfo, error)

	err := fs.Chmod(path, perm)
	checkPathError(t, err, "Chmod")

	err = fs.Chtimes(path, time.Now(), time.Now())
	checkPathError(t, err, "Chtimes")

	// fs.Create doesn't return an error

	err = fs.Mkdir(path2, perm)
	if err != nil {
		t.Error(err)
	}
	err = fs.Mkdir(path2, perm)
	checkPathError(t, err, "Mkdir")

	err = fs.MkdirAll(path3, perm)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.Open(path)
	checkPathError(t, err, "Open")

	_, err = fs.OpenFile(path, os.O_RDWR, perm)
	checkPathError(t, err, "OpenFile")

	err = fs.Remove(path)
	checkPathError(t, err, "Remove")

	err = fs.RemoveAll(path)
	if err != nil {
		t.Error("RemoveAll:", err)
	}

	err = fs.Rename(path, path2)
	checkLinkError(t, err, "Rename")

	_, err = fs.Stat(path)
	checkPathError(t, err, "Stat")
}

func checkLinkError(t *testing.T, err error, op string) {
	t.Helper()
	linkErr, ok := err.(*os.LinkError)
	if !ok {
		t.Error(op+":", err, "is not a os.LinkError")
		return
	}
	_, ok = linkErr.Err.(*os.LinkError)
	if ok {
		t.Error(op+":", err, "contains another os.LinkError")
	}
}

func checkPathError(t *testing.T, err error, op string) {
	t.Helper()
	pathErr, ok := err.(*os.PathError)
	if !ok {
		t.Error(op+":", err, "is not a os.PathError")
		return
	}
	_, ok = pathErr.Err.(*os.PathError)
	if ok {
		t.Error(op+":", err, "contains another os.PathError")
	}
}

func TestMemFsRename(t *testing.T) {
	memFs := &MemMapFs{}

	const (
		oldPath        = "/old"
		newPath        = "/prefix/new"
		fileName       = "afero.txt"
		subDirName     = "subdir"
		subDirFileName = "subafero.txt"
	)
	err := memFs.Mkdir(oldPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	err = memFs.Rename(oldPath, newPath)
	if err == nil {
		t.Fatal("Missing parent dir for new path should return an error")
	}
	err = memFs.Mkdir("/prefix", 0700)
	if err != nil {
		t.Fatal(err)
	}

	oldFilePath := filepath.Join(oldPath, fileName)
	_, err = memFs.Create(oldFilePath)
	if err != nil {
		t.Fatal(err)
	}
	oldSubDirPath := filepath.Join(oldPath, subDirName)
	err = memFs.Mkdir(oldSubDirPath, 0700)
	if err != nil {
		t.Fatal(err)
	}
	oldSubDirFilePath := filepath.Join(oldPath, subDirName, subDirFileName)
	_, err = memFs.Create(oldSubDirFilePath)
	if err != nil {
		t.Fatal(err)
	}

	err = memFs.Rename(oldPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("MemFs contents:")
	memFs.List()
	fmt.Println()

	newFilePath := filepath.Join(newPath, fileName)
	newSubDirFilePath := filepath.Join(newPath, subDirName, subDirFileName)
	_, err = memFs.Stat(newFilePath)
	if err != nil {
		t.Errorf("File should exist in new directory %q but received error: %s", newFilePath, err)
	}
	_, err = memFs.Stat(newSubDirFilePath)
	if err != nil {
		t.Errorf("File should exist in new sub directory %q but received error: %s", newSubDirFilePath, err)
	}

	_, err = memFs.Stat(oldFilePath)
	if err == nil {
		t.Errorf("File should not exist in old directory %q", oldFilePath)
	}
	_, err = memFs.Stat(oldSubDirFilePath)
	if err == nil {
		t.Errorf("File should not exist in old sub directory %q", oldSubDirFilePath)
	}
}

// Ensure os.O_EXCL is correctly handled.
func TestOpenFileExcl(t *testing.T) {
	const fileName = "/myFileTest"
	const fileMode = os.FileMode(0765)

	fs := NewMemMapFs()

	// First creation should succeed.
	f, err := fs.OpenFile(fileName, os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		t.Errorf("OpenFile Create Excl failed: %s", err)
		return
	}
	f.Close()

	// Second creation should fail.
	_, err = fs.OpenFile(fileName, os.O_CREATE|os.O_EXCL, fileMode)
	if err == nil {
		t.Errorf("OpenFile Create Excl should have failed, but it didn't")
	}
	checkPathError(t, err, "Open")
}

// Ensure Permissions are set on OpenFile/Mkdir/MkdirAll
func TestPermSet(t *testing.T) {
	const fileName = "/myFileTest"
	const dirPath = "/myDirTest"
	const dirPathAll = "/my/path/to/dir"

	const fileMode = os.FileMode(0765)
	// directories will also have the directory bit set
	const dirMode = fileMode | os.ModeDir

	fs := NewMemMapFs()

	// Test Openfile
	f, err := fs.OpenFile(fileName, os.O_CREATE, fileMode)
	if err != nil {
		t.Errorf("OpenFile Create failed: %s", err)
		return
	}
	f.Close()

	s, err := fs.Stat(fileName)
	if err != nil {
		t.Errorf("Stat failed: %s", err)
		return
	}
	if s.Mode().String() != fileMode.String() {
		t.Errorf("Permissions Incorrect: %s != %s", s.Mode().String(), fileMode.String())
		return
	}

	// Test Mkdir
	err = fs.Mkdir(dirPath, dirMode)
	if err != nil {
		t.Errorf("MkDir Create failed: %s", err)
		return
	}
	s, err = fs.Stat(dirPath)
	if err != nil {
		t.Errorf("Stat failed: %s", err)
		return
	}
	// sets File
	if s.Mode().String() != dirMode.String() {
		t.Errorf("Permissions Incorrect: %s != %s", s.Mode().String(), dirMode.String())
		return
	}

	// Test MkdirAll
	err = fs.MkdirAll(dirPathAll, dirMode)
	if err != nil {
		t.Errorf("MkDir Create failed: %s", err)
		return
	}
	s, err = fs.Stat(dirPathAll)
	if err != nil {
		t.Errorf("Stat failed: %s", err)
		return
	}
	if s.Mode().String() != dirMode.String() {
		t.Errorf("Permissions Incorrect: %s != %s", s.Mode().String(), dirMode.String())
		return
	}
}

// Fails if multiple file objects use the same file.at counter in MemMapFs
func TestMultipleOpenFiles(t *testing.T) {
	defer removeAllTestFiles(t)
	const fileName = "afero-demo2.txt"

	var data = make([][]byte, len(Fss))

	for i, fs := range Fss {
		dir := testDir(fs)
		path := filepath.Join(dir, fileName)
		fh1, err := fs.Create(path)
		if err != nil {
			t.Error("fs.Create failed: " + err.Error())
		}
		_, err = fh1.Write([]byte("test"))
		if err != nil {
			t.Error("fh.Write failed: " + err.Error())
		}
		_, err = fh1.Seek(0, os.SEEK_SET)
		if err != nil {
			t.Error(err)
		}

		fh2, err := fs.OpenFile(path, os.O_RDWR, 0777)
		if err != nil {
			t.Error("fs.OpenFile failed: " + err.Error())
		}
		_, err = fh2.Seek(0, os.SEEK_END)
		if err != nil {
			t.Error(err)
		}
		_, err = fh2.Write([]byte("data"))
		if err != nil {
			t.Error(err)
		}
		err = fh2.Close()
		if err != nil {
			t.Error(err)
		}

		_, err = fh1.Write([]byte("data"))
		if err != nil {
			t.Error(err)
		}
		err = fh1.Close()
		if err != nil {
			t.Error(err)
		}
		// the file now should contain "datadata"
		data[i], err = ReadFile(fs, path)
		if err != nil {
			t.Error(err)
		}
	}

	for i, fs := range Fss {
		if i == 0 {
			continue
		}
		if string(data[0]) != string(data[i]) {
			t.Errorf("%s and %s don't behave the same\n"+
				"%s: \"%s\"\n%s: \"%s\"\n",
				Fss[0].Name(), fs.Name(), Fss[0].Name(), data[0], fs.Name(), data[i])
		}
	}
}

// Test if file.Write() fails when opened as read only
func TestReadOnly(t *testing.T) {
	defer removeAllTestFiles(t)
	const fileName = "afero-demo.txt"

	for _, fs := range Fss {
		dir := testDir(fs)
		path := filepath.Join(dir, fileName)

		f, err := fs.Create(path)
		if err != nil {
			t.Error(fs.Name()+":", "fs.Create failed: "+err.Error())
		}
		_, err = f.Write([]byte("test"))
		if err != nil {
			t.Error(fs.Name()+":", "Write failed: "+err.Error())
		}
		f.Close()

		f, err = fs.Open(path)
		if err != nil {
			t.Error("fs.Open failed: " + err.Error())
		}
		_, err = f.Write([]byte("data"))
		if err == nil {
			t.Error(fs.Name()+":", "No write error")
		}
		f.Close()

		f, err = fs.OpenFile(path, os.O_RDONLY, 0644)
		if err != nil {
			t.Error("fs.Open failed: " + err.Error())
		}
		_, err = f.Write([]byte("data"))
		if err == nil {
			t.Error(fs.Name()+":", "No write error")
		}
		f.Close()
	}
}

func TestWriteCloseTime(t *testing.T) {
	defer removeAllTestFiles(t)
	const fileName = "afero-demo.txt"

	for _, fs := range Fss {
		dir := testDir(fs)
		path := filepath.Join(dir, fileName)

		f, err := fs.Create(path)
		if err != nil {
			t.Error(fs.Name()+":", "fs.Create failed: "+err.Error())
		}
		f.Close()

		f, err = fs.Create(path)
		if err != nil {
			t.Error(fs.Name()+":", "fs.Create failed: "+err.Error())
		}
		fi, err := f.Stat()
		if err != nil {
			t.Error(fs.Name()+":", "Stat failed: "+err.Error())
		}
		timeBefore := fi.ModTime()

		// sorry for the delay, but we have to make sure time advances,
		// also on non Un*x systems...
		switch runtime.GOOS {
		case "windows":
			time.Sleep(2 * time.Second)
		case "darwin":
			time.Sleep(1 * time.Second)
		default: // depending on the FS, this may work with < 1 second, on my old ext3 it does not
			time.Sleep(1 * time.Second)
		}

		_, err = f.Write([]byte("test"))
		if err != nil {
			t.Error(fs.Name()+":", "Write failed: "+err.Error())
		}
		f.Close()
		fi, err = fs.Stat(path)
		if err != nil {
			t.Error(fs.Name()+":", "fs.Stat failed: "+err.Error())
		}
		if fi.ModTime().Equal(timeBefore) {
			t.Error(fs.Name()+":", "ModTime was not set on Close()")
		}
	}
}

// This test should be run with the race detector on:
// go test -race -v -timeout 10s -run TestRacingDeleteAndClose
func TestRacingDeleteAndClose(t *testing.T) {
	fs := NewMemMapFs()
	pathname := "testfile"
	f, err := fs.Create(pathname)
	if err != nil {
		t.Fatal(err)
	}

	in := make(chan bool)

	go func() {
		<-in
		f.Close()
	}()
	go func() {
		<-in
		fs.Remove(pathname)
	}()
	close(in)
}

// This test should be run with the race detector on:
// go test -run TestMemFsDataRace -race
func TestMemFsDataRace(t *testing.T) {
	const dir = "test_dir"
	fs := NewMemMapFs()

	if err := fs.MkdirAll(dir, 0777); err != nil {
		t.Fatal(err)
	}

	const n = 1000
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			fname := filepath.Join(dir, fmt.Sprintf("%d.txt", i))
			if err := WriteFile(fs, fname, []byte(""), 0777); err != nil {
				panic(err)
			}
			if err := fs.Remove(fname); err != nil {
				panic(err)
			}
		}
	}()

loop:
	for {
		select {
		case <-done:
			break loop
		default:
			_, err := ReadDir(fs, dir)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestMemFsDirMode(t *testing.T) {
	fs := NewMemMapFs()
	err := fs.Mkdir("/testDir1", 0644)
	if err != nil {
		t.Error(err)
	}
	err = fs.MkdirAll("/sub/testDir2", 0644)
	if err != nil {
		t.Error(err)
	}
	info, err := fs.Stat("/testDir1")
	if err != nil {
		t.Error(err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
	if !info.Mode().IsDir() {
		t.Error("FileMode is not directory")
	}
	info, err = fs.Stat("/sub/testDir2")
	if err != nil {
		t.Error(err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
	if !info.Mode().IsDir() {
		t.Error("FileMode is not directory")
	}
}

func TestMemFsUnexpectedEOF(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()

	if err := WriteFile(fs, "file.txt", []byte("abc"), 0777); err != nil {
		t.Fatal(err)
	}

	f, err := fs.Open("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Seek beyond the end.
	_, err = f.Seek(512, 0)
	if err != nil {
		t.Fatal(err)
	}

	buff := make([]byte, 256)
	_, err = io.ReadAtLeast(f, buff, 256)

	if err != io.ErrUnexpectedEOF {
		t.Fatal("Expected ErrUnexpectedEOF")
	}
}

// https://github.com/spf13/afero/issues/149
func TestMemFsMkdirWithoutParent(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	err := fs.Mkdir("/a/b/c", 0700)
	if !os.IsNotExist(err) {
		t.Error("Mkdir should fail if parent directory does not exist:", err)
	}
	pathErr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("Mkdir error should be a path error, found: %T", err)
	}
	if pathErr.Op != "mkdir" {
		t.Error("Invalid op for mkdir error:", pathErr.Op)
	}
	if pathErr.Path != "/a/b/c" {
		// path errors should be the same as OsFs, which is the passed in path and not a parent path
		t.Error("Invalid path for mkdir error:", pathErr.Path)
	}

	_, err = fs.Create("/a")
	if err != nil {
		t.Fatal(err)
	}

	err = fs.Mkdir("/a/b", 0700)
	if !IsNotDir(err) {
		t.Error("Mkdir should fail if parent is not a directory:", err)
	}
	pathErr, ok = err.(*os.PathError)
	if !ok {
		t.Fatalf("Mkdir error should be a path error, found: %T", err)
	}
	if pathErr.Op != "mkdir" {
		t.Error("Invalid op for mkdir error:", pathErr.Op)
	}
	if pathErr.Path != "/a/b" {
		// path errors should be the same as OsFs, which is the passed in path and not a parent path
		t.Error("Invalid path for mkdir error:", pathErr.Path)
	}
}

func TestMemFsCreateWithoutParent(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	_, err := fs.Create("/a/b/c")
	if !os.IsNotExist(err) {
		t.Error("Create should fail if parent directory does not exist:", err)
	}

	_, err = fs.Create("/a")
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.Create("/a/b")
	if !IsNotDir(err) {
		t.Error("Create should fail if parent is not a directory:", err)
	}
	pathErr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("Create error should be a path error, found: %T", err)
	}
	if pathErr.Op != "open" {
		t.Error("Invalid op for create ('open') error:", pathErr.Op)
	}
	if pathErr.Path != "/a/b" {
		// path errors should be the same as OsFs, which is the passed in path and not a parent path
		t.Error("Invalid path for create error:", pathErr.Path)
	}
}

func TestMemFsRemoveNonEmptyDir(t *testing.T) {
	memFs := &MemMapFs{}

	err := memFs.MkdirAll("/a/b", 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, err = memFs.Create("/a/b/c")
	if err != nil {
		t.Fatal(err)
	}
	err = memFs.Remove("/a/b")
	if !IsNotEmpty(err) {
		t.Errorf("Removing intermediate directory should fail with 'not empty': %s", err)
	}
}

func TestMemFsChmod(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	const file = "/hello"
	if err := fs.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().String() != "drwx------" {
		t.Fatal("mkdir failed to create a directory: mode =", info.Mode())
	}

	err = fs.Chmod(file, 0)
	if err != nil {
		t.Error("Failed to run chmod:", err)
	}

	info, err = fs.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().String() != "d---------" {
		t.Error("chmod should not change file type. New mode =", info.Mode())
	}
}

func TestMemFsRootPerm(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	info, err := fs.Stat("/")
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != os.ModeDir|0755 {
		t.Error("Root '/' must be a directory with 755 permissions, found:", info.Mode())
	}
}

func TestMemFsIllegalMkdir(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	err := fs.Mkdir("/foo", os.ModeSocket|0755)
	if err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat("/foo")
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != os.ModeDir|0755 {
		t.Error("Mkdir must only set permission bits:", info.Mode())
	}
}

func TestMemFsIllegalMkdirAll(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	err := fs.MkdirAll("/foo/bar", os.ModeSocket|0755)
	if err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat("/foo")
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != os.ModeDir|0755 {
		t.Error("MkdirAll must only set permission bits:", info.Mode())
	}

	info, err = fs.Stat("/foo/bar")
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != os.ModeDir|0755 {
		t.Error("MkdirAll must only set permission bits:", info.Mode())
	}
}

func TestMemFsIllegalOpenFile(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	_, err := fs.OpenFile("/foo", os.O_CREATE, os.ModeSocket|0755)
	if err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat("/foo")
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != 0755 {
		t.Error("OpenFile must only set permission bits:", info.Mode())
	}
}

func TestMemFsCreatePerm(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	_, err := fs.Create("/foo")
	if err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat("/foo")
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode() != 0666 {
		t.Error("Create for new files should set permission to 0666:", info.Mode())
	}
}

func TestMemFsCreateIsDir(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	if err := fs.Mkdir("/foo", 0700); err != nil {
		t.Fatal(err)
	}

	_, err := fs.Create("/foo")
	pathErr, ok := err.(*os.PathError)
	if !ok {
		t.Fatal("Error is not a path error", err)
	}
	if pathErr.Op != "open" {
		t.Error("PathError.Op should be 'open':", pathErr.Op)
	}
	if pathErr.Path != "/foo" {
		t.Error("PathError.Path should be '/foo':", pathErr.Path)
	}
	if pathErr.Err != ErrIsDir {
		t.Error("PathError.Err should be 'ErrIsDir':", pathErr.Err)
	}
}

func TestMemFsMkdirModTime(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	err := fs.Mkdir("/foo", 0700)
	if err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat("/foo")
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(info.ModTime())
	if elapsed > 10*time.Second {
		t.Error("Mod time should be close to now, but time apart was", elapsed)
	}
}

func TestMemFsChmodNormalizePath(t *testing.T) {
	t.Parallel()

	fs := NewMemMapFs()
	const file = "hello"
	if err := fs.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}

	err := fs.Chmod(file, 0)
	if err != nil {
		t.Error("Failed to run chmod:", err)
	}
}

func TestMemFsMkdirAllNotDir(t *testing.T) {
	fs := NewMemMapFs()

	_, err := fs.Create("/a")
	if err != nil {
		t.Fatal(err)
	}

	err = fs.MkdirAll("/a/b", 0700)
	if !IsNotDir(err) {
		t.Error("MkdirAll should fail if parent is not a directory:", err)
	}
	pathErr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("MkdirAll error should be a path error, found: %T", err)
	}
	if pathErr.Op != "mkdir" {
		t.Error("Invalid op for mkdirall (mkdir op) error:", pathErr.Op)
	}
	if pathErr.Path != "/a" {
		// path errors should be the same as OsFs, which is the passed in path and not a parent path
		t.Error("Invalid path for mkdirall error:", pathErr.Path)
	}
}

func TestMemFsOpenFileCreateExistingDir(t *testing.T) {
	fs := NewMemMapFs()

	err := fs.Mkdir("/a", 0755)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.OpenFile("/a", os.O_CREATE|os.O_RDWR, 0755)
	if err == nil {
		t.Fatal("OpenFile on a directory (non read-only) should fail")
	}

	if !IsDirErr(err) {
		t.Error("Error must be ErrIsDir, got", err)
	}
	pathErr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("Error type must be os.PathError: %T", err)
	}
	if pathErr.Op != "open" {
		t.Error("PathError.Op should be open, got", pathErr.Op)
	}
	if pathErr.Path != "/a" {
		t.Error("PathError.Path should be /a, got", pathErr.Path)
	}
}

func TestMemFsOpenFileExistingDir(t *testing.T) {
	fs := NewMemMapFs()

	err := fs.Mkdir("/a", 0755)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.OpenFile("/a", os.O_RDONLY, 0000)
	if err != nil {
		t.Error("OpenFile on a directory with read-only should succeed, got:", err)
	}
}

func TestMemFsOpenFileCreateExistingFile(t *testing.T) {
	fs := NewMemMapFs()

	_, err := fs.Create("/a")
	if err != nil {
		t.Fatal(err)
	}
	const (
		originalFilePerm = os.FileMode(0700)
		openFilePerm     = os.FileMode(0755)
	)
	err = fs.Chmod("/a", originalFilePerm)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.OpenFile("/a", os.O_CREATE|os.O_RDWR, openFilePerm)
	if err != nil {
		t.Fatal("Unexpected error on OpenFile create for an existing file", err)
	}

	info, err := fs.Stat("/a")
	if err != nil {
		t.Fatal("Unexpected error stat'ing openfile:", err)
	}

	if info.Mode().Perm() != originalFilePerm {
		t.Errorf("File permissions should not change on existing file. Should be %s, but found: %s", originalFilePerm.String(), info.Mode().Perm().String())
	}
}

func TestMemFsOpenFileTruncateReadOnly(t *testing.T) {
	fs := NewMemMapFs()

	f, err := fs.Create("/a")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = fs.OpenFile("/a", os.O_TRUNC, 0700)
	if err != nil {
		t.Fatal(err)
	}
	info, err := fs.Stat("/a")
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != 0 {
		t.Error("Truncate on read-only settings should work. Actual size after truncate open:", info.Size())
	}
}
