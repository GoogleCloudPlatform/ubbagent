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
	"sync"
)

// Type memoryPersistence is a Persistence implementation that stores values and queues json-encoded
// data in an in-memory map. This implementations does not offer persistence across restarts.
type memoryPersistence struct {
	items map[string][]byte
	mutex sync.RWMutex
}

// NewMemoryPersistence constructs a new Persistence that stores objects in memory.
func NewMemoryPersistence() Persistence {
	return newMemoryPersistence()
}

func newMemoryPersistence() *memoryPersistence {
	var mp memoryPersistence
	mp.items = make(map[string][]byte)
	return &mp
}

func (p *memoryPersistence) Value(name string) Value {
	return &lockingValue{p.value(name)}
}

func (p *memoryPersistence) Queue(name string) Queue {
	return &valueQueue{p.value(name)}
}

func (p *memoryPersistence) value(name string) *memoryValue {
	return &memoryValue{p: p, name: name}
}

type memoryValue struct {
	p    *memoryPersistence
	name string
}

func (v *memoryValue) mutex() *sync.RWMutex {
	return &v.p.mutex
}

func (v *memoryValue) load(obj interface{}) error {
	data, exists := v.p.items[v.name]
	if !exists {
		return ErrNotFound
	}
	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}
	return nil
}

func (v *memoryValue) store(obj interface{}) error {
	if buff, err := json.Marshal(obj); err != nil {
		return err
	} else {
		v.p.items[v.name] = buff
	}
	return nil
}

func (v *memoryValue) remove() error {
	if _, ok := v.p.items[v.name]; !ok {
		return ErrNotFound
	}
	delete(v.p.items, v.name)
	return nil
}
