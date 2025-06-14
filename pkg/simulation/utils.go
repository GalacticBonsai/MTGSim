// Package simulation provides utility functions for MTG simulation.
package simulation

import (
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// SliceGet returns the element at index and a new slice with that element removed.
func SliceGet[T any](slice []T, index int) (T, []T) {
	var out T
	if index < 0 || index >= len(slice) {
		return out, slice
	}
	if len(slice) == 0 {
		return out, slice
	}
	out = slice[index]
	slice = append(slice[:index], slice[index+1:]...)
	return out, slice
}

// GetRandom returns a random element from a non-empty slice. Panics if slice is empty.
func GetRandom[T any](slice []T) T {
	if len(slice) == 0 {
		panic("GetRandom called with empty slice")
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return slice[r.Intn(len(slice))]
}

// GetDecks recursively finds all files in a directory and its subdirectories.
func GetDecks(dir string) ([]string, error) {
	var fileList []string
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			subDirFiles, err := GetDecks(filepath.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			fileList = append(fileList, subDirFiles...)
			continue
		}
		fileList = append(fileList, filepath.Join(dir, file.Name()))
	}
	return fileList, nil
}
