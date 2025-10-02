package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/zeebo/blake3"
)

type Config struct {
	dryRun          bool
	referentDirs    []string
	threadCount     int
	removalStrategy string
	verboseOutput   bool
}

func main() {
	if err := createRootCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func createRootCommand() *cobra.Command {
	config := &Config{}

	rootCmd := &cobra.Command{
		Use:   "remove-duplicates [sources]",
		Short: "Remove duplicate files by hash",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			removeDuplicatesFromDirectories(args, config)
		},
	}

	addFlags(rootCmd, config)
	return rootCmd
}

func addFlags(cmd *cobra.Command, config *Config) {
	cmd.Flags().BoolVarP(&config.dryRun, "dry-run", "n", false, "Dry run, do not delete any files")
	cmd.Flags().StringSliceVarP(&config.referentDirs, "referent", "r", nil, "Optional referent directories (comma-separated)")
	cmd.Flags().IntVarP(&config.threadCount, "threads", "t", 1, "Number of threads to use for hashing")
	cmd.Flags().StringVarP(&config.removalStrategy, "remove-by", "m", "oldest", "Removal method: newest, oldest, interactive")
	cmd.Flags().BoolVarP(&config.verboseOutput, "verbose", "v", false, "Show verbose output during hashing")
}

func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	if fileInfo.Mode() == fs.ModeSymlink {
		return "", fmt.Errorf("file is a symlink: %s", filePath)
	}

	hasher := blake3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	hashBytes := hasher.Sum(nil)
	return fmt.Sprintf("%x", hashBytes), nil
}

func createFileHashMap(files []string, config *Config) map[string][]string {
	hashToFiles := make(map[string][]string)
	var mutex sync.Mutex
	var waitGroup sync.WaitGroup

	fileChan := make(chan string, len(files))
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	for range config.threadCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			processFilesFromChannel(fileChan, hashToFiles, &mutex, config.verboseOutput)
		}()
	}

	waitGroup.Wait()
	return hashToFiles
}

func processFilesFromChannel(fileChan <-chan string, hashToFiles map[string][]string, mutex *sync.Mutex, verbose bool) {
	for filePath := range fileChan {
		hash, err := calculateFileHash(filePath)
		if err != nil {
			fmt.Printf("Error hashing file %s: %v\n", filePath, err)
			continue
		}

		if verbose {
			fmt.Printf("Hashing file: %s, Hash: %s\n", filePath, hash)
		}

		mutex.Lock()
		hashToFiles[hash] = append(hashToFiles[hash], filePath)
		mutex.Unlock()
	}
}

func processAndRemoveDuplicates(fileHashes, referentHashes map[string][]string, config *Config) {
	for hash, files := range fileHashes {
		if hasReferentMatch(hash, referentHashes) {
			removeAllFilesWithHash(files, "due to referent match", config.dryRun)
			continue
		}

		if len(files) <= 1 {
			continue
		}

		filesToRemove := selectFilesForRemoval(files, hash, config.removalStrategy)
		removeFiles(filesToRemove, config.dryRun)
	}
}

func hasReferentMatch(hash string, referentHashes map[string][]string) bool {
	_, exists := referentHashes[hash]
	return exists
}

func removeAllFilesWithHash(files []string, reason string, isDryRun bool) {
	for _, file := range files {
		if isDryRun {
			fmt.Printf("Would remove (%s): %s\n", reason, file)
		} else {
			fmt.Printf("Removing (%s): %s\n", reason, file)
			os.Remove(file)
		}
	}
}

func selectFilesForRemoval(files []string, hash string, strategy string) []string {
	switch strategy {
	case "newest":
		return selectNewestFiles(files)
	case "oldest":
		return selectOldestFiles(files)
	case "interactive":
		return selectFilesInteractively(files, hash)
	default:
		return selectOldestFiles(files)
	}
}

func selectNewestFiles(files []string) []string {
	sortedFiles := make([]string, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		fileInfoI, _ := os.Stat(sortedFiles[i])
		fileInfoJ, _ := os.Stat(sortedFiles[j])
		return fileInfoI.ModTime().After(fileInfoJ.ModTime())
	})
	return sortedFiles[1:]
}

func selectOldestFiles(files []string) []string {
	sortedFiles := make([]string, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		fileInfoI, _ := os.Stat(sortedFiles[i])
		fileInfoJ, _ := os.Stat(sortedFiles[j])
		return fileInfoI.ModTime().Before(fileInfoJ.ModTime())
	})
	return sortedFiles[1:]
}

func selectFilesInteractively(files []string, hash string) []string {
	fmt.Printf("Duplicates found for hash %s:\n", hash)
	displayFileOptions(files)

	fmt.Println("Select the file(s) to remove by entering the corresponding numbers (comma-separated, or 'a' for all except the first):")
	var input string
	fmt.Scanln(&input)

	if input == "a" {
		return files[1:]
	}

	return selectFilesByIndices(files, input)
}

func displayFileOptions(files []string) {
	for i, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			fmt.Printf("Error getting file info for %s: %v\n", file, err)
			continue
		}
		modTime := fileInfo.ModTime().Format("2006-01-02 15:04:05")
		fmt.Printf("[%d] %s (Modified: %s)\n", i, file, modTime)
	}
}

func selectFilesByIndices(files []string, input string) []string {
	var selectedFiles []string
	indices := parseCommaSeparatedIntegers(input)
	for _, index := range indices {
		if index >= 0 && index < len(files) {
			selectedFiles = append(selectedFiles, files[index])
		}
	}
	return selectedFiles
}

func removeFiles(files []string, isDryRun bool) {
	for _, file := range files {
		if isDryRun {
			fmt.Printf("Would remove: %s\n", file)
		} else {
			fmt.Printf("Removing: %s\n", file)
			os.Remove(file)
		}
	}
}

func parseCommaSeparatedIntegers(input string) []int {
	var indices []int
	for _, s := range strings.Split(input, ",") {
		i, err := strconv.Atoi(strings.TrimSpace(s))
		if err == nil {
			indices = append(indices, i)
		}
	}
	return indices
}

func collectFilesFromDirectories(directories []string) ([]string, error) {
	var allFiles []string
	for _, directory := range directories {
		files, err := collectFilesFromSingleDirectory(directory)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}
	return allFiles, nil
}

func collectFilesFromSingleDirectory(directory string) ([]string, error) {
	var files []string
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func removeDuplicatesFromDirectories(sourceDirectories []string, config *Config) {
	if len(sourceDirectories) == 0 {
		fmt.Println("Please provide directories to search for duplicates")
		return
	}

	referentFiles, err := collectFilesFromDirectories(config.referentDirs)
	if err != nil {
		fmt.Printf("Error collecting referent files: %v\n", err)
		return
	}

	sourceFiles, err := collectFilesFromDirectories(sourceDirectories)
	if err != nil {
		fmt.Printf("Error collecting source files: %v\n", err)
		return
	}

	referentHashes := createFileHashMap(referentFiles, config)
	sourceFileHashes := createFileHashMap(sourceFiles, config)

	processAndRemoveDuplicates(sourceFileHashes, referentHashes, config)
}
