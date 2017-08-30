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

// Type diskPersistence is a Persistence implementation that stores values and queues as json text
// files in a hierarchy under a specified filesystem directory.
type diskPersistence struct {
	directory string
	mutex     sync.RWMutex
}

// NewDiskPersistence creates a diskPersistence that stores data under the given filesystem
// directory.
func NewDiskPersistence(directory string) (Persistence, error) {
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return nil, errors.New("persistence: could not create directory: " + directory + ": " + err.Error())
	}
	return &diskPersistence{directory: directory}, nil
}

func (p *diskPersistence) Value(name string) Value {
	return &lockingValue{&diskValue{p: p, name: name}}
}

func (p *diskPersistence) Queue(name string) Queue {
	return &valueQueue{&diskValue{p: p, name: name}}
}

type diskValue struct {
	p    *diskPersistence
	name string
}

func (v *diskValue) mutex() *sync.RWMutex {
	return &v.p.mutex
}

func (v *diskValue) load(obj interface{}) error {
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

func (v *diskValue) store(obj interface{}) error {
	var jsontext []byte
	var err error
	if jsontext, err = json.Marshal(obj); err != nil {
		return err
	}
	filename := v.jsonFile(v.name)
	dirname := path.Dir(filename)

	if err = os.MkdirAll(dirname, directoryMode); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filename, jsontext, fileMode); err != nil {
		return err
	}
	return nil
}

func (v *diskValue) remove() error {
	filename := v.jsonFile(v.name)
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
