// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package persistence

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

type emptyStruct struct{}

func TestEmptyFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_endpoint_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	ioutil.WriteFile(path.Join(tmpdir, "empty.json"), []byte{}, 0644)
	p, err := NewDiskPersistence(tmpdir)
	if err != nil {
		t.Fatalf("Unable to create new DiskPersistence")
	}
	v := emptyStruct{}
	err = p.Value("empty").Load(&v)
	if err != ErrNotFound {
		t.Fatalf("Expected NotFound error but found %+v", err)
	}
}
