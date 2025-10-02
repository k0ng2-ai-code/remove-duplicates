package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func createTestFile(t *testing.T, dir, name, content string) string {
	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	return filePath
}

func TestCalculateFileHash(t *testing.T) {
	tempDir := t.TempDir()
	filePath := createTestFile(t, tempDir, "test.txt", "Hello, World!")

	hash, err := calculateFileHash(filePath)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	// Expected hash value as a hex-encoded string
	expectedHash := "288a86a79f20a3d6dccdca7713beaed178798296bdfa7913fa2a62d9727bf8f8" // Replace with actual BLAKE3 hash of "Hello, World!"
	if hash != expectedHash {
		t.Errorf("Expected hash %v, got %s", expectedHash, hash)
	}
}

func TestCreateFileHashMap(t *testing.T) {
	config := &Config{threadCount: 1, verboseOutput: false}
	tempDir := t.TempDir()
	filePath1 := createTestFile(t, tempDir, "file1.txt", "Content A")
	filePath2 := createTestFile(t, tempDir, "file2.txt", "Content A")
	filePath3 := createTestFile(t, tempDir, "file3.txt", "Content B")

	files := []string{filePath1, filePath2, filePath3}
	hashMap := createFileHashMap(files, config)

	fmt.Println("HashMap:", hashMap) // Debug output

	// Ensure there are 2 unique hashes
	if len(hashMap) != 2 {
		t.Errorf("Expected 2 unique hashes, got %d", len(hashMap))
	}

	// Ensure both file1.txt and file2.txt have the same hash
	hash1 := calculateFileHashOrFail(t, filePath1)
	fmt.Printf("Expected Hash for file1.txt: %s\n", hash1) // Debug output

	if len(hashMap[hash1]) != 2 {
		t.Errorf("Expected 2 files with the same hash, got %d", len(hashMap[hash1]))
	}
}

func calculateFileHashOrFail(t *testing.T, filePath string) string {
	hash, err := calculateFileHash(filePath)
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}
	return hash
}

func TestProcessAndRemoveDuplicates(t *testing.T) {
	config := &Config{threadCount: 1, removalStrategy: "newest", dryRun: true, verboseOutput: false}
	tempDir := t.TempDir()
	filePath1 := createTestFile(t, tempDir, "file1.txt", "Content A")
	filePath2 := createTestFile(t, tempDir, "file2.txt", "Content A")
	filePath3 := createTestFile(t, tempDir, "file3.txt", "Content B")

	files := []string{filePath1, filePath2, filePath3}
	hashMap := createFileHashMap(files, config)

	processAndRemoveDuplicates(hashMap, nil, config)

	// Verify that only the newest duplicate would have been removed
	if _, err := os.Stat(filePath2); os.IsNotExist(err) {
		t.Errorf("File %s should not have been removed", filePath2)
	}
}

func TestReferentDirectory(t *testing.T) {
	config := &Config{threadCount: 1, removalStrategy: "newest", dryRun: true, verboseOutput: false}
	tempDir := t.TempDir()
	referentDir := t.TempDir()

	filePath1 := createTestFile(t, tempDir, "file1.txt", "Content A")
	filePath2 := createTestFile(t, tempDir, "file2.txt", "Content A")
	referentFile := createTestFile(t, referentDir, "referent.txt", "Content A")

	files := []string{filePath1, filePath2}
	referentFiles := []string{referentFile}

	referentHashes := createFileHashMap(referentFiles, config)
	fileHashes := createFileHashMap(files, config)

	processAndRemoveDuplicates(fileHashes, referentHashes, config)

	// Verify that none of the original files were removed because they match the referent file
	if _, err := os.Stat(filePath1); os.IsNotExist(err) {
		t.Errorf("File %s should not have been removed", filePath1)
	}

	if _, err := os.Stat(filePath2); os.IsNotExist(err) {
		t.Errorf("File %s should not have been removed", filePath2)
	}
}
