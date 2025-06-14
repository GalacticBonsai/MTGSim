package simulation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSliceGet(t *testing.T) {
	// Test with string slice
	slice := []string{"a", "b", "c", "d"}
	
	// Test getting element at valid index
	element, newSlice := SliceGet(slice, 1)
	if element != "b" {
		t.Errorf("Expected element 'b', got '%s'", element)
	}
	if len(newSlice) != 3 {
		t.Errorf("Expected new slice length 3, got %d", len(newSlice))
	}
	expected := []string{"a", "c", "d"}
	for i, v := range expected {
		if newSlice[i] != v {
			t.Errorf("Expected new slice %v, got %v", expected, newSlice)
			break
		}
	}

	// Test getting element at first index
	element, newSlice = SliceGet(slice, 0)
	if element != "a" {
		t.Errorf("Expected element 'a', got '%s'", element)
	}
	if len(newSlice) != 3 {
		t.Errorf("Expected new slice length 3, got %d", len(newSlice))
	}

	// Test getting element at last index
	element, newSlice = SliceGet(slice, 3)
	if element != "d" {
		t.Errorf("Expected element 'd', got '%s'", element)
	}
	if len(newSlice) != 3 {
		t.Errorf("Expected new slice length 3, got %d", len(newSlice))
	}

	// Test with invalid index (negative)
	element, newSlice = SliceGet(slice, -1)
	if element != "" {
		t.Errorf("Expected empty element for invalid index, got '%s'", element)
	}
	if len(newSlice) != len(slice) {
		t.Errorf("Expected original slice unchanged for invalid index")
	}

	// Test with invalid index (too large)
	element, newSlice = SliceGet(slice, 10)
	if element != "" {
		t.Errorf("Expected empty element for invalid index, got '%s'", element)
	}
	if len(newSlice) != len(slice) {
		t.Errorf("Expected original slice unchanged for invalid index")
	}

	// Test with empty slice
	emptySlice := []string{}
	element, newSlice = SliceGet(emptySlice, 0)
	if element != "" {
		t.Errorf("Expected empty element from empty slice, got '%s'", element)
	}
	if len(newSlice) != 0 {
		t.Errorf("Expected empty slice to remain empty")
	}
}

func TestSliceGetWithInts(t *testing.T) {
	slice := []int{10, 20, 30, 40, 50}
	
	element, newSlice := SliceGet(slice, 2)
	if element != 30 {
		t.Errorf("Expected element 30, got %d", element)
	}
	if len(newSlice) != 4 {
		t.Errorf("Expected new slice length 4, got %d", len(newSlice))
	}
	expected := []int{10, 20, 40, 50}
	for i, v := range expected {
		if newSlice[i] != v {
			t.Errorf("Expected new slice %v, got %v", expected, newSlice)
			break
		}
	}
}

func TestGetRandom(t *testing.T) {
	slice := []string{"a", "b", "c", "d", "e"}
	
	// Test that GetRandom returns an element from the slice
	for i := 0; i < 10; i++ {
		element := GetRandom(slice)
		found := false
		for _, v := range slice {
			if v == element {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetRandom returned element '%s' not in slice %v", element, slice)
		}
	}

	// Test with single element slice
	singleSlice := []string{"only"}
	element := GetRandom(singleSlice)
	if element != "only" {
		t.Errorf("Expected 'only', got '%s'", element)
	}
}

func TestGetRandomPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected GetRandom to panic with empty slice")
		}
	}()
	
	emptySlice := []string{}
	GetRandom(emptySlice)
}

func TestGetDecks(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	
	// Create some test files and directories
	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	
	// Create test files
	files := []string{
		filepath.Join(tempDir, "deck1.deck"),
		filepath.Join(tempDir, "deck2.txt"),
		filepath.Join(subDir, "deck3.deck"),
		filepath.Join(subDir, "deck4.txt"),
	}
	
	for _, file := range files {
		err := os.WriteFile(file, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}
	
	// Test GetDecks
	foundFiles, err := GetDecks(tempDir)
	if err != nil {
		t.Fatalf("GetDecks failed: %v", err)
	}
	
	if len(foundFiles) != 4 {
		t.Errorf("Expected 4 files, got %d", len(foundFiles))
	}
	
	// Check that all expected files are found
	expectedFiles := make(map[string]bool)
	for _, file := range files {
		expectedFiles[file] = true
	}
	
	for _, foundFile := range foundFiles {
		if !expectedFiles[foundFile] {
			t.Errorf("Unexpected file found: %s", foundFile)
		}
		delete(expectedFiles, foundFile)
	}
	
	if len(expectedFiles) > 0 {
		t.Errorf("Some expected files were not found: %v", expectedFiles)
	}
}

func TestGetDecksNonExistentDir(t *testing.T) {
	_, err := GetDecks("/non/existent/directory")
	if err == nil {
		t.Errorf("Expected error for non-existent directory")
	}
}
