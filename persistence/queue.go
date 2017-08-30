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
)

// Queue implements a simple persistent queue. Each Queue function is threadsafe and atomic within
// the scope of the Persistence instance that created it. However, there is no guarantee that
// multiple Queue function calls operate atomically. E.g., if a caller calls Peek and then later
// Dequeue, there's a chance that some other caller will have modified the queue in the meantime.
type Queue interface {
	// Peek loads the object at the head of this Queue into obj. If successful, nil is returned and
	// obj will be populated. ErrNotFound is returned if the queue is empty or does not exist. Other
	// I/O errors may be returned in the event of I/O failures.
	Peek(obj interface{}) error

	// Dequeue removes the front of this Queue. If successful, nil is returned. ErrNotFound is
	// returned if the queue is empty or does not exist. Other I/O errors may be returned in the event
	// of I/O failures. If obj is non-nil, it will contain removed value upon success.
	Dequeue(obj interface{}) error

	// Enqueue stores obj at the back of this Queue. Returns nil if the object was stored, or an error
	// if something failed.
	Enqueue(obj interface{}) error
}

// Type valueQueue is a Queue that stores its state within a single value. Queue state is stored as
// a json array.
type valueQueue struct {
	value value
}

func (vq *valueQueue) Peek(obj interface{}) error {
	var queue []json.RawMessage
	// Grab the value's associated persistence read lock and load the queue
	vq.value.mutex().RLock()
	err := vq.value.load(&queue)
	vq.value.mutex().RUnlock()
	if err != nil {
		return err
	}
	// If the queue exists but is somehow empty, we return ErrNotFound
	if len(queue) == 0 {
		return ErrNotFound
	}
	// Unmarshal the front of the queue and store it in obj.
	if err := json.Unmarshal(queue[0], obj); err != nil {
		return err
	}
	return nil
}

func (vq *valueQueue) Dequeue(obj interface{}) error {
	var queue []json.RawMessage
	// Grab the value's associated persistence lock
	vq.value.mutex().Lock()
	defer vq.value.mutex().Unlock()
	// First, load the existing queue
	if err := vq.value.load(&queue); err != nil {
		return err
	}
	// If the queue exists but is somehow empty, we return ErrNotFound
	if len(queue) == 0 {
		return ErrNotFound
	}
	// If the caller passed a non-nil obj, store the value currently at the front of the queue.
	if obj != nil {
		if err := json.Unmarshal(queue[0], obj); err != nil {
			return err
		}
	}
	// Remove the front. If the new queue still has entries, store it. Else, remove the backing value.
	newq := queue[1:]
	if len(newq) > 0 {
		if err := vq.value.store(queue[1:]); err != nil {
			return err
		}
	} else {
		if err := vq.value.remove(); err != nil {
			return err
		}
	}
	return nil
}

func (vq *valueQueue) Enqueue(obj interface{}) error {
	var queue []json.RawMessage
	var err error
	var bytes []byte
	// First marshal the given object into json text.
	if bytes, err = json.Marshal(obj); err != nil {
		return err
	}
	// Grab the value's associated persistence lock
	vq.value.mutex().Lock()
	defer vq.value.mutex().Unlock()
	// Load the existing queue in preparation for updating
	if err = vq.value.load(&queue); err != nil && err != ErrNotFound {
		return err
	}
	// Append the new value to the end of the queue, and store the result.
	queue = append(queue, bytes)
	if err := vq.value.store(queue); err != nil {
		return err
	}
	return nil
}
