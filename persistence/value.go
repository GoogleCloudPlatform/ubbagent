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

import "sync"

// An internal value whose load, store, and remove operations don't acquire locks. Locking must be
// done by an additional layer.
type value interface {
	load(obj interface{}) error
	store(obj interface{}) error
	remove() error
	mutex() *sync.RWMutex
}

// lockingValue is a Value type that wraps an internal non-locking value and acquires locks around
// calls to load, store, and remove.
type lockingValue struct {
	v value
}

// Load acquires the value's read lock and calls its load function.
func (v *lockingValue) Load(obj interface{}) error {
	v.v.mutex().RLock()
	defer v.v.mutex().RUnlock()
	return v.v.load(obj)
}

// Store acquires the value's write lock and calls its store function.
func (v *lockingValue) Store(obj interface{}) error {
	v.v.mutex().Lock()
	defer v.v.mutex().Unlock()
	return v.v.store(obj)
}

// Remove acquires the value's write lock and calls its remove function.
func (v *lockingValue) Remove() error {
	v.v.mutex().Lock()
	defer v.v.mutex().Unlock()
	return v.v.remove()
}
