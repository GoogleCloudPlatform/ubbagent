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
	"reflect"
	"testing"
)

type Outer struct {
	Value1 int
	Value2 int
	Foo    Inner
}

type Inner struct {
	ValueMap map[string]string
}

func TestMemoryPersistence(t *testing.T) {
	testPersistence(NewMemoryPersistence(), t)
}

func TestDiskPersistence(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "persistence_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)
	p, err := NewDiskPersistence(tmpdir)
	if err != nil {
		t.Fatalf("Unexpected error creating DiskPersistence: %+v", err)
	}
	testPersistence(p, t)
}

func testPersistence(p Persistence, t *testing.T) {
	var input1 []Outer
	input1 = append(input1, Outer{
		Value1: 0,
		Value2: 10,
		Foo: Inner{
			ValueMap: map[string]string{
				"foo": "bar",
			},
		},
	})
	input1 = append(input1, Outer{
		Value1: 10,
		Value2: 20,
		Foo: Inner{
			ValueMap: map[string]string{
				"foo": "baz",
			},
		},
	})

	input2 := Outer{
		Value1: 333,
		Value2: 444,
		Foo: Inner{
			ValueMap: map[string]string{
				"baz": "foo",
			},
		},
	}

	if err := p.Store("test/input1", input1); err != nil {
		t.Fatalf("Unexpected error storing input1: %+v", err)
	}

	// We can also persist by passing in a pointer
	if err := p.Store("test/input2", &input2); err != nil {
		t.Fatalf("Unexpected error storing input2: %+v", err)
	}

	var output1 []Outer
	var output2 Outer
	var output3 Outer

	if err := p.Load("test/input1", &output1); err != nil {
		t.Fatalf("Unexpected error loading input1: %+v", err)
	}
	if err := p.Load("test/input2", &output2); err != nil {
		t.Fatalf("Unexpected error loading input2: %+v", err)
	}

	if !reflect.DeepEqual(input1, output1) {
		t.Fatalf("output1: expected: %+v, got: %+v", input1, output1)
	}

	if !reflect.DeepEqual(input2, output2) {
		t.Fatalf("output2: expected: %+v, got: %+v", input2, output2)
	}

	// Test overwriting
	if err := p.Store("test/input1", input2); err != nil {
		t.Fatalf("Unexpected error storing input1: %+v", err)
	}
	if err := p.Load("test/input1", &output3); err != nil {
		t.Fatalf("Unexpected error loading input1: %+v", err)
	}

	if !reflect.DeepEqual(input2, output3) {
		t.Fatalf("output3: expected: %+v, got: %+v", input2, output3)
	}
}
