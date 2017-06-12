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
	// Load loads the object with the given name into the object referenced by obj. The name parameter
	// should not contain a file extension.
	// Returns nil if an object with the given name existed and was successfully copied into obj.
	// Returns ErrNotFound if an object with the given name does not exist.
	// Returns some other I/O error if an object with the given name exists, but loading failed.
	Load(name string, obj interface{}) error

	// Store stores the given object with the given name. The name parameter should not contain a file
	// extension. Returns nil if the object was stored, or an error if something failed.
	Store(name string, obj interface{}) error
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

func (p *diskPersistence) Load(name string, obj interface{}) error {
	jsontext, err := p.loadBytes(name)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsontext, obj)
	if err != nil {
		return err
	}
	return nil
}

func (p *diskPersistence) loadBytes(name string) ([]byte, error) {
	filename := p.jsonFile(name)
	p.mutex.RLock()
	defer p.mutex.RUnlock()
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

func (p *diskPersistence) Store(name string, obj interface{}) error {
	var jsontext []byte
	var err error
	if jsontext, err = json.Marshal(obj); err != nil {
		return err
	}
	filename := p.jsonFile(name)
	dirname := path.Dir(filename)

	p.mutex.Lock()
	defer p.mutex.Unlock()
	if err = os.MkdirAll(dirname, directoryMode); err != nil {
		return err
	}
	if err = ioutil.WriteFile(filename, jsontext, fileMode); err != nil {
		return err
	}
	return nil
}

func (p *diskPersistence) jsonFile(name string) string {
	return path.Join(p.directory, name+".json")
}

type memoryPersistence struct {
	items map[string][]byte
	mutex sync.RWMutex
}

func (p *memoryPersistence) Load(name string, obj interface{}) error {
	p.mutex.RLock()
	data, exists := p.items[name]
	p.mutex.RUnlock()
	if !exists {
		return ErrNotFound
	}
	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}
	return nil
}

func (p *memoryPersistence) Store(name string, obj interface{}) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if buff, err := json.Marshal(obj); err != nil {
		return err
	} else {
		p.items[name] = buff
	}
	return nil
}
