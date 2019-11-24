package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin"
)

type file struct {
	path     string
	modified time.Time
	blocks   int64
}

func main() {
	var (
		path             string
		minBlocksFreePct int
		minFilesFreePct  int
	)

	kingpin.Arg("path", "Path to clean").Required().StringVar(&path)
	kingpin.Flag("min-size-pct", "Percentage of space to keep free").Default("25").IntVar(&minBlocksFreePct)
	kingpin.Flag("min-files-pct", "Percentage of files (inodes) to keep free").Default("25").IntVar(&minFilesFreePct)
	kingpin.Parse()

	var fs syscall.Statfs_t
	err := syscall.Statfs(path, &fs)
	if err != nil {
		fmt.Println("Getting fs size:", err)
		os.Exit(1)
	}

	minBlocksFree := int64(fs.Blocks) * int64(minBlocksFreePct) / 100
	minFilesFree := int64(fs.Files) * int64(minFilesFreePct) / 100

	if int64(fs.Bavail) > minBlocksFree && int64(fs.Ffree) > minFilesFree {
		return
	}

	needBlocks := minBlocksFree - int64(fs.Bavail)
	needFiles := minFilesFree - int64(fs.Ffree)

	var files []file
	err = filepath.Walk(path, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}

		files = append(files, file{
			path:     path,
			modified: fi.ModTime(),
			blocks:   1 + fi.Size()/int64(fs.Bsize),
		})

		return nil
	})
	if err != nil {
		fmt.Println("Walking:", err)
		os.Exit(1)
	}

	sort.Slice(files, func(a, b int) bool {
		return files[a].modified.Before(files[b].modified)
	})

	for _, f := range files {
		if needFiles <= 0 && needBlocks <= 0 {
			break
		}
		if err := os.Remove(f.path); err != nil {
			fmt.Println("Cleaning:", err)
			continue
		}
		needBlocks -= f.blocks
		needFiles--
	}
}
