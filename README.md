# remove-duplicates

`remove-duplicates` is a command-line tool written in Go that identifies and removes duplicate files based on their BLAKE3 hash. The tool allows you to compare files across multiple directories and supports advanced features like using referent directories, interactive deletion, and multi-threaded hashing.

## Features

- **Hashing with BLAKE3:** Uses the BLAKE3 hash algorithm for fast and secure file comparison.
- **Referent Directory Support:** Specify one or more referent directories. If a file in the referent directory has the same hash as a file in the source directory, the file in the source directory is removed.
- **Removal Methods:** Choose between removing the newest, oldest, or select files interactively.
- **Dry Run Mode:** Preview the files that would be removed without actually deleting them.
- **Multithreading:** Specify the number of threads to use for faster hashing.
- **Verbose Mode:** Output detailed information during the hashing process, including file paths and their corresponding hashes.

## Installation

### Option 1: Install with `go install`

If you have Go installed, you can easily install `remove-duplicates` using the `go install` command:

```bash
go install github.com/k0ng2-ai-code/remove-duplicates@latest
```

This will install the binary in your `$GOPATH/bin` directory, making it available system-wide.

### Option 2: Manual Installation

1. **Clone the repository:**
    ```bash
    git clone https://github.com/k0ng2-ai-code/remove-duplicates.git
    cd remove-duplicates
    ```

2. **Build the binary:**
    ```bash
    go build -o remove-duplicates
    ```

3. **Run the tool:**
    ```bash
    ./remove-duplicates [flags] [source directories...]
    ```

## Usage

```bash
./remove-duplicates [flags] [source directories...]
```

### Flags

- `-d, --dry-run`: Perform a dry run, do not delete any files.
- `-r, --referent`: Optional referent directories (comma-separated).
- `-t, --threads`: Number of threads to use for hashing. Default is 1.
- `-m, --remove-by`: Specify which duplicate files to remove:
  - `newest`: Removes all duplicates except the newest file.
  - `oldest`: Removes all duplicates except the oldest file.
  - `interactive`: Allows you to manually select which duplicates to remove.
- `-v, --verbose`: Show verbose output during hashing.

### Examples

- **Remove duplicates by newest file in source directories:**
    ```bash
    ./remove-duplicates -m newest /path/to/source1 /path/to/source2
    ```

- **Remove duplicates interactively, with detailed output:**
    ```bash
    ./remove-duplicates -m interactive -v /path/to/source1 /path/to/source2
    ```

- **Dry run to see what would be removed without actual deletion:**
    ```bash
    ./remove-duplicates -d -m oldest /path/to/source1 /path/to/source2
    ```

- **Use referent directories to remove matching files from source directories:**
    ```bash
    ./remove-duplicates -r /path/to/referent -m newest /path/to/source1 /path/to/source2
    ```
