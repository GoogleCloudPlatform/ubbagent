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
	p := NewMemoryPersistence()
	testPersistence(p, t)
	testQueue(p.Queue("test_queue"), t)
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
	testQueue(p.Queue("test_queue"), t)
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

	if err := p.Value("test/input1").Store(input1); err != nil {
		t.Fatalf("Unexpected error storing input1: %+v", err)
	}

	// We can also persist by passing in a pointer
	if err := p.Value("test/input2").Store(&input2); err != nil {
		t.Fatalf("Unexpected error storing input2: %+v", err)
	}

	var output1 []Outer
	var output2 Outer
	var output3 Outer

	if err := p.Value("test/input1").Load(&output1); err != nil {
		t.Fatalf("Unexpected error loading input1: %+v", err)
	}
	if err := p.Value("test/input2").Load(&output2); err != nil {
		t.Fatalf("Unexpected error loading input2: %+v", err)
	}

	if !reflect.DeepEqual(input1, output1) {
		t.Fatalf("output1: expected: %+v, got: %+v", input1, output1)
	}

	if !reflect.DeepEqual(input2, output2) {
		t.Fatalf("output2: expected: %+v, got: %+v", input2, output2)
	}

	// Test overwriting
	if err := p.Value("test/input1").Store(input2); err != nil {
		t.Fatalf("Unexpected error storing input1: %+v", err)
	}
	if err := p.Value("test/input1").Load(&output3); err != nil {
		t.Fatalf("Unexpected error loading input1: %+v", err)
	}

	if !reflect.DeepEqual(input2, output3) {
		t.Fatalf("output3: expected: %+v, got: %+v", input2, output3)
	}

	// Test remove
	if err := p.Value("test/input1").Remove(); err != nil {
		t.Fatalf("Unexpected error removing input1: %+v", err)
	}
	if err := p.Value("test/input1").Load(&output1); err != ErrNotFound {
		t.Fatalf("Expected ErrNotFound when loading removed input1, got: %+v", err)
	}
	if err := p.Value("test/input1").Remove(); err != ErrNotFound {
		t.Fatalf("Expected ErrNotFound when removing already-removed input1, got: %+v", err)
	}
}

func testQueue(q Queue, t *testing.T) {
	type value struct {
		A int
		B string
	}

	value1 := value{A: 1, B: "foo1"}
	value2 := value{A: 2, B: "foo2"}
	value3 := value{A: 3, B: "foo3"}

	if err := q.Enqueue(&value1); err != nil {
		t.Fatalf("Unexpected error adding queue value 1: %+v", err)
	}
	if err := q.Enqueue(&value2); err != nil {
		t.Fatalf("Unexpected error adding queue value 2: %+v", err)
	}

	v := value{}
	if err := q.Peek(&v); err != nil {
		t.Fatalf("Unexpected error getting queue value 1: %+v", err)
	}
	if !reflect.DeepEqual(v, value1) {
		t.Fatalf("Unexpected value for value 1: %+v", v)
	}
	// Try again without removing the head.
	if err := q.Peek(&v); err != nil {
		t.Fatalf("Unexpected error getting queue value 1: %+v", err)
	}
	if !reflect.DeepEqual(v, value1) {
		t.Fatalf("Unexpected value for value 1: %+v", v)
	}
	if err := q.Dequeue(nil); err != nil {
		t.Fatalf("Unexpected error removing head: %+v", err)
	}

	if err := q.Peek(&v); err != nil {
		t.Fatalf("Unexpected error getting queue value 2: %+v", err)
	}
	if !reflect.DeepEqual(v, value2) {
		t.Fatalf("Unexpected value for value 2: %+v", v)
	}

	if err := q.Enqueue(&value3); err != nil {
		t.Fatalf("Unexpected error adding queue value 3: %+v", err)
	}

	// At this point we should still have value 2 and value 3 in the queue.
	if err := q.Peek(&v); err != nil {
		t.Fatalf("Unexpected error getting queue value 2: %+v", err)
	}
	if !reflect.DeepEqual(v, value2) {
		t.Fatalf("Unexpected value for value 2: %+v", v)
	}
	if err := q.Dequeue(nil); err != nil {
		t.Fatalf("Unexpected error removing head: %+v", err)
	}

	v2 := value{}
	if err := q.Peek(&v); err != nil {
		t.Fatalf("Unexpected error getting queue value 3: %+v", err)
	}
	if !reflect.DeepEqual(v, value3) {
		t.Fatalf("Unexpected value for value 3: %+v", v)
	}
	if err := q.Dequeue(&v2); err != nil {
		t.Fatalf("Unexpected error removing head: %+v", err)
	}
	if !reflect.DeepEqual(v2, value3) {
		t.Fatalf("Unexpected value for value 3: %+v", v)
	}

	// The queue should be empty now. Both Head and RemoveHead should return ErrNotFound
	if err := q.Dequeue(nil); err != ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %+v", err)
	}
	if err := q.Peek(&v); err != ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %+v", err)
	}
}
