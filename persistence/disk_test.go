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
	"reflect"
	"testing"
)

type testStruct struct {
	Value int
}

func TestEmptyFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_endpoint_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Make sure that it starts clean, and a newly stored data can be retrieved.
	testBrandNewDiskPersistenceCanStoreAndRetrieve(t, tmpdir, "empty", testStruct{Value: 10})

	// Overwrite the state file to an empty file.
	ioutil.WriteFile(path.Join(tmpdir, "empty.json"), []byte{}, 0644)
	p, err := NewDiskPersistence(tmpdir)
	if err != nil {
		t.Fatalf("Unable to create new DiskPersistence")
	}
	var v testStruct
	err = p.Value("empty").Load(&v)
	// If we wrote the wrong file, the error would be nil.
	// If the library couldn't handle empty file, the error wouldn't be ErrNotFound.
	if err != ErrNotFound {
		t.Fatalf("Expected NotFound error but found %+v", err)
	}
}

func TestFileExternallyModified(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_endpoint_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Make sure that it starts clean, and a newly stored data can be retrieved.
	expectedValue := testStruct{Value: 10}
	p := testBrandNewDiskPersistenceCanStoreAndRetrieve(t, tmpdir, "empty", expectedValue)

	// Overwrite the state file to an empty file. Since we still use the same
	// persistence object, this shouldn't affect its memory.
	ioutil.WriteFile(path.Join(tmpdir, "empty.json"), []byte{}, 0644)
	var actualValue testStruct
	err = p.Value("empty").Load(&actualValue)
	if err != nil {
		t.Fatalf("Unexpected error %+v", err)
	}
	if !reflect.DeepEqual(expectedValue, actualValue) {
		t.Fatalf("Loaded value is %+v while expecting %+v", actualValue, expectedValue)
	}
}

func TestRestoredFromFile(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_endpoint_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	expectedValue := testStruct{Value: 10}
	testBrandNewDiskPersistenceCanStoreAndRetrieve(t, tmpdir, "key", expectedValue)

	p, err := NewDiskPersistence(tmpdir)
	if err != nil {
		t.Fatalf("Failed to recreate new disk persistenc %+v", err)
	}
	var actualValue testStruct
	if err = p.Value("key").Load(&actualValue); err != nil {
		t.Fatalf("Failed to load value %+v", err)
	}
	if !reflect.DeepEqual(expectedValue, actualValue) {
		t.Fatalf("Loaded value is %+v while expecting a restored value of %+v", actualValue, expectedValue)
	}
}

func testBrandNewDiskPersistenceCanStoreAndRetrieve(t *testing.T, tmpdir string, expectedKey string, expectedValue testStruct) (p Persistence) {
	var actualValue testStruct

	// Make sure that it starts clean, and a newly stored data can be retrieved.
	p, err := NewDiskPersistence(tmpdir)
	if err != nil {
		t.Fatalf("Failed to create new disk persistenc %+v", err)
	}
	if err = p.Value(expectedKey).Load(&actualValue); err != ErrNotFound {
		t.Fatalf("Expected NotFound error but found %+v", err)
	}
	if err = p.Value(expectedKey).Store(expectedValue); err != nil {
		t.Fatalf("Failed to store value %+v", err)
	}
	actualValue.Value = 0
	if err = p.Value(expectedKey).Load(&actualValue); err != nil {
		t.Fatalf("Failed to load value %+v", err)
	}
	if !reflect.DeepEqual(expectedValue, actualValue) {
		t.Fatalf("Loaded value is %+v while expecting %+v", actualValue, expectedValue)
	}

	return
}
