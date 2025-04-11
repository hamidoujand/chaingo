package disk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hamidoujand/chaingo/database"
)

var ErrEOC = errors.New("end of chain")

// Disk represents the storage engine for reading, writing blocks into disk.
type Disk struct {
	path string
}

func New(path string) (*Disk, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("mkdirAll: %w", err)
	}

	return &Disk{path: path}, nil
}

// Close implements the database.Storage interface.
func (d *Disk) Close() error {
	return nil
}

// Write writes a blockData into disk.
func (d *Disk) Write(blockData database.BlockData) error {
	bs, err := json.MarshalIndent(blockData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}

	//create a file per block
	p := d.getPath(blockData.Header.Number)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("openFile: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(bs); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func (d *Disk) GetBlock(num uint64) (database.BlockData, error) {
	f, err := os.OpenFile(d.getPath(num), os.O_RDONLY, 0600)
	if err != nil {
		return database.BlockData{}, fmt.Errorf("openFile: %w", err)
	}

	defer f.Close()

	var blockData database.BlockData
	if err := json.NewDecoder(f).Decode(&blockData); err != nil {
		return database.BlockData{}, fmt.Errorf("decode json: %w", err)
	}

	return blockData, nil
}

func (d *Disk) Reset() error {
	if err := os.RemoveAll(d.path); err != nil {
		return fmt.Errorf("removeAll: %w", err)
	}

	if err := os.MkdirAll(d.path, 0755); err != nil {
		return fmt.Errorf("mkdirAll: %w", err)
	}

	return nil
}

func (d *Disk) ForEach() database.Iterator {
	return &DiskIterator{storage: d}
}

func (d *Disk) getPath(number uint64) string {
	name := strconv.FormatUint(number, 10)
	return filepath.Join(d.path, fmt.Sprintf("%s.json", name))
}

// ==============================================================================
type DiskIterator struct {
	storage *Disk
	current uint64
	eoc     bool //end of chain
}

func (di *DiskIterator) Next() (database.BlockData, error) {
	if di.eoc {
		return database.BlockData{}, ErrEOC
	}

	di.current++
	blockData, err := di.storage.GetBlock(di.current)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			di.eoc = true
		}
	}
	return blockData, err
}

func (di *DiskIterator) Done() bool {
	return di.eoc
}
