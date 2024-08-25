package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zeebo/blake3"
)

var (
	dryRun    bool
	referents []string
	threads   int
	removeBy  string
	verbose   bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "remove-duplicates [sources]",
		Short: "Remove duplicate files by hash",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			execute(args)
		},
	}

	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run, do not delete any files")
	rootCmd.Flags().StringSliceVarP(&referents, "referent", "r", nil, "Optional referent directories (comma-separated)")
	rootCmd.Flags().IntVarP(&threads, "threads", "t", 1, "Number of threads to use for hashing")
	rootCmd.Flags().StringVarP(&removeBy, "remove-by", "m", "newest", "Removal method: newest, oldest, interactive")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose output during hashing")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func computeHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hasher := blake3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

func hashFiles(files []string) map[string][]string {
	hashMap := make(map[string][]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	fileChan := make(chan string, len(files))
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				hash, err := computeHash(file)
				if err != nil {
					fmt.Printf("Error hashing file %s: %v\n", file, err)
					continue
				}
				hashString := fmt.Sprintf("%x", hash) // Convert hash to string for map key
				if verbose {
					fmt.Printf("Hashing file: %s, Hash: %s\n", file, hashString)
				}
				mu.Lock()
				hashMap[hashString] = append(hashMap[hashString], file)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return hashMap
}

func removeDuplicates(fileHashes, referentHashes map[string][]string) {
	for hash, files := range fileHashes {
		if _, exists := referentHashes[hash]; exists {
			// If a file in the referent directory has the same hash, remove all files with that hash from the source
			for _, file := range files {
				if dryRun {
					fmt.Printf("Would remove (due to referent match): %s\n", file)
				} else {
					fmt.Printf("Removing (due to referent match): %s\n", file)
					os.Remove(file)
				}
			}
			continue
		}

		if len(files) <= 1 {
			continue
		}

		var toRemove []string

		switch removeBy {
		case "newest":
			sort.Slice(files, func(i, j int) bool {
				fi, _ := os.Stat(files[i])
				fj, _ := os.Stat(files[j])
				return fi.ModTime().After(fj.ModTime())
			})
			toRemove = files[1:]
		case "oldest":
			sort.Slice(files, func(i, j int) bool {
				fi, _ := os.Stat(files[i])
				fj, _ := os.Stat(files[j])
				return fi.ModTime().Before(fj.ModTime())
			})
			toRemove = files[1:]
		case "interactive":
			fmt.Printf("Duplicates found for hash %s:\n", hash)
			for i, file := range files {
				fi, err := os.Stat(file)
				if err != nil {
					fmt.Printf("Error getting file info for %s: %v\n", file, err)
					continue
				}
				modTime := fi.ModTime().Format("2006-01-02 15:04:05")
				fmt.Printf("[%d] %s (Modified: %s)\n", i, file, modTime)
			}

			fmt.Println("Select the file(s) to remove by entering the corresponding numbers (comma-separated, or 'a' for all except the first):")
			var input string
			fmt.Scanln(&input)

			if input == "a" {
				toRemove = files[1:]
			} else {
				indices := parseInput(input)
				for _, index := range indices {
					if index >= 0 && index < len(files) {
						toRemove = append(toRemove, files[index])
					}
				}
			}
		}

		for _, file := range toRemove {
			if dryRun {
				fmt.Printf("Would remove: %s\n", file)
			} else {
				fmt.Printf("Removing: %s\n", file)
				os.Remove(file)
			}
		}
	}
}

func parseInput(input string) []int {
	var indices []int
	for _, s := range strings.Split(input, ",") {
		i, err := strconv.Atoi(s)
		if err == nil {
			indices = append(indices, i)
		}
	}
	return indices
}

func gatherFiles(dirs []string) ([]string, error) {
	var files []string
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func execute(args []string) {
	if len(args) == 0 {
		fmt.Println("Please provide directories to search for duplicates")
		return
	}

	// Gather files from referent directories
	referentFiles, err := gatherFiles(referents)
	if err != nil {
		fmt.Printf("Error gathering referent files: %v\n", err)
		return
	}

	// Gather files from other directories
	files, err := gatherFiles(args)
	if err != nil {
		fmt.Printf("Error gathering files: %v\n", err)
		return
	}

	// Hash files in referent directories
	referentHashes := hashFiles(referentFiles)

	// Hash files in other directories
	fileHashes := hashFiles(files)

	// Compare and remove duplicates only from non-referent directories
	removeDuplicates(fileHashes, referentHashes)
}
