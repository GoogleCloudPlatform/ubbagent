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
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"sync"
)

const (
	fileMode      = 0644 // Mode bits used when creating files
	directoryMode = 0755 // Mode bits used when creating directories
)

// ErrNotFound is returned by a Persistence's Load operation when an object with the given name
// is not found.
var ErrNotFound = errors.New("object not found")

// Persistence provides simple support for loading and storing structures. Persistence
// implementations are threadsafe.
type Persistence interface {
	// Value creates a Value instance associated with the given name. Names should not contain file
	// extensions.
	Value(name string) Value

	// Queue creates a Queue instance associated with the given name. Names should not contain file
	// extensions.
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

// Queue implements a simple persistent queue.
type Queue interface {
	// Head loads the object at the head of this Queue into obj. If successful, nil is returned and
	// obj will be populated. ErrNotFound is returned if the queue is empty or does not exist. Other
	// I/O errors may be returned in the event of I/O failures.
	Head(obj interface{}) error

	// RemoveHead removes the head of this Queue. If successful, nil is returned. ErrNotFound is
	// returned if the queue is empty or does not exist. Other I/O errors may be returned in the event
	// of I/O failures.
	RemoveHead() error

	// Push stores obj at the tail of this Queue. Returns nil if the object was stored, or an error if
	// something failed.
	Push(obj interface{}) error
}

// NewDiskPersistence constructs a new Persistence that stores objects as json files under the given
// directory.
func NewDiskPersistence(directory string) (Persistence, error) {
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return nil, errors.New("persistence: could not create directory: " + directory + ": " + err.Error())
	}
	return &diskPersistence{directory: directory}, nil
}

// NewMemoryPersistence constructs a new Persistence that stores objects in memory.
func NewMemoryPersistence() Persistence {
	var mp memoryPersistence
	mp.items = make(map[string][]byte)
	return &mp
}

type diskPersistence struct {
	directory string
	mutex     sync.RWMutex
}

func (p *diskPersistence) Value(name string) Value {
	return &diskValue{p: p, name: name}
}

func (p *diskPersistence) Queue(name string) Queue {
	return &valueQueue{value: p.Value(name)}
}

type diskValue struct {
	p    *diskPersistence
	name string
}

func (v *diskValue) Load(obj interface{}) error {
	jsontext, err := v.loadBytes(v.name)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsontext, obj)
	if err != nil {
		return err
	}
	return nil
}

func (v *diskValue) Store(obj interface{}) error {
	var jsontext []byte
	var err error
	if jsontext, err = json.Marshal(obj); err != nil {
		return err
	}
	filename := v.jsonFile(v.name)
	dirname := path.Dir(filename)

	v.p.mutex.Lock()
	defer v.p.mutex.Unlock()
	if err = os.MkdirAll(dirname, directoryMode); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filename, jsontext, fileMode); err != nil {
		return err
	}
	return nil
}

func (v *diskValue) Remove() error {
	filename := v.jsonFile(v.name)
	v.p.mutex.Lock()
	defer v.p.mutex.Unlock()
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (v *diskValue) loadBytes(name string) ([]byte, error) {
	filename := v.jsonFile(name)
	v.p.mutex.RLock()
	defer v.p.mutex.RUnlock()
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// object doesn't exist
		return nil, ErrNotFound
	}
	jsontext, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return jsontext, nil
}

func (v *diskValue) jsonFile(name string) string {
	return path.Join(v.p.directory, name+".json")
}

type memoryPersistence struct {
	items map[string][]byte
	mutex sync.RWMutex
}

func (p *memoryPersistence) Value(name string) Value {
	return &memoryValue{p: p, name: name}
}

func (p *memoryPersistence) Queue(name string) Queue {
	return &valueQueue{value: p.Value(name)}
}

type memoryValue struct {
	p    *memoryPersistence
	name string
}

func (v *memoryValue) Load(obj interface{}) error {
	v.p.mutex.RLock()
	data, exists := v.p.items[v.name]
	v.p.mutex.RUnlock()
	if !exists {
		return ErrNotFound
	}
	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}
	return nil
}

func (v *memoryValue) Store(obj interface{}) error {
	v.p.mutex.Lock()
	defer v.p.mutex.Unlock()
	if buff, err := json.Marshal(obj); err != nil {
		return err
	} else {
		v.p.items[v.name] = buff
	}
	return nil
}

func (v *memoryValue) Remove() error {
	v.p.mutex.Lock()
	defer v.p.mutex.Unlock()
	if _, ok := v.p.items[v.name]; !ok {
		return ErrNotFound
	}
	delete(v.p.items, v.name)
	return nil
}

type valueQueue struct {
	value Value
	mutex sync.RWMutex
}

func (vq *valueQueue) Head(obj interface{}) error {
	var queue []json.RawMessage
	vq.mutex.RLock()
	err := vq.value.Load(&queue)
	vq.mutex.RUnlock()
	if err != nil {
		return err
	}
	if len(queue) == 0 {
		return ErrNotFound
	}
	if err := json.Unmarshal(queue[0], obj); err != nil {
		return err
	}
	return nil
}

func (vq *valueQueue) RemoveHead() error {
	var queue []json.RawMessage
	vq.mutex.Lock()
	defer vq.mutex.Unlock()
	if err := vq.value.Load(&queue); err != nil {
		return err
	}
	if len(queue) == 0 {
		return ErrNotFound
	}
	newq := queue[1:]
	if len(newq) > 0 {
		if err := vq.value.Store(queue[1:]); err != nil {
			return err
		}
	} else {
		if err := vq.value.Remove(); err != nil {
			return err
		}
	}
	return nil
}

func (vq *valueQueue) Push(obj interface{}) error {
	var queue []json.RawMessage
	var err error
	var bytes []byte
	if bytes, err = json.Marshal(obj); err != nil {
		return err
	}
	vq.mutex.Lock()
	defer vq.mutex.Unlock()
	if err = vq.value.Load(&queue); err != nil && err != ErrNotFound {
		return err
	}
	queue = append(queue, bytes)
	if err := vq.value.Store(queue); err != nil {
		return err
	}
	return nil
}
