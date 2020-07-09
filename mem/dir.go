// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mem

import (
	"os"
	"syscall"
)

type Dir interface {
	Len() int
	Names() []string
	Files() []*FileData
	Add(*FileData)
	Remove(*FileData)
}

func RemoveFromMemDir(dir *FileData, f *FileData) {
	dir.memDir.Remove(f)
}

func AddToMemDir(dir *FileData, f *FileData) {
	dir.memDir.Add(f)
}

func ReadMemDir(dir *FileData) ([]os.FileInfo, error) {
	if !dir.dir {
		return nil, &os.PathError{Op: "readdir", Path: dir.name, Err: syscall.ENOTDIR}
	}
	dir.Lock()
	files := dir.memDir.Files()
	dir.Unlock()
	fileInfos := make([]os.FileInfo, len(files))
	for i := range files {
		fileInfos[i] = &FileInfo{files[i]}
	}
	return fileInfos, nil
}

func InitializeDir(d *FileData) {
	if d.memDir == nil {
		d.dir = true
		d.memDir = &DirMap{}
	}
}
