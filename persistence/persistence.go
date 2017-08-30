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

import "errors"

const (
	fileMode      = 0644 // Mode bits used when creating files
	directoryMode = 0755 // Mode bits used when creating directories
)

// ErrNotFound is returned by a Persistence's Load operation when an object with the given name
// is not found.
var ErrNotFound = errors.New("object not found")

// Persistence provides simple support for loading and storing structures. A single Persistence
// instance and all of the Queue and Value instances that it returns are threadsafe. However, no
// such guarantee exists if multiple Persistence instances share the same backing store (e.g., the
// same filesystem directory).
type Persistence interface {
	// Value creates a Value instance associated with the given name. Names should not contain file
	// extensions. Within the scope of a single Persistence instance, Value can be called multiple
	// times with the same name and all returned instances will operate on the same data in a
	// threadsafe manner.
	Value(name string) Value

	// Queue creates a Queue instance associated with the given name. Names should not contain file
	// extensions. Within the scope of a single Persistence instance, Queue can be called multiple
	// times with the same name and all returned instances will operate on the same data in a
	// threadsafe manner.
	Queue(name string) Queue
}

// Value stores and loads a single value.
type Value interface {
	// Load loads the object stored by this Value into obj. If successful, nil is returned and
	// obj will be populated. ErrNotFound is returned if the Value is empty or does not exist. Other
	// I/O errors may be returned in the event of I/O failures.
	Load(obj interface{}) error

	// Store stores obj into this Value. Returns nil if the object was stored,
	// or an error if something failed.
	Store(obj interface{}) error

	// Remove removes this Value from persistence. Remove returns nil if the value was removed,
	// ErrNotFound if the value did not exist, or some other I/O error if one occurred during removal.
	Remove() error
}
