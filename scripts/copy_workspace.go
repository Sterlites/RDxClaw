package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	// Source is root workspace directory relative to cmd/rdxclaw
	src := "../../workspace"
	// Destination is cmd/rdxclaw/workspace
	dst := "workspace"

	// Check if source exists
	if _, err := os.Stat(src); os.IsNotExist(err) {
		fmt.Printf("Source directory %s does not exist\n", src)
		os.Exit(1)
	}

	// Remove destination if it exists
	if err := os.RemoveAll(dst); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error removing destination %s: %v\n", dst, err)
		os.Exit(1)
	}

	// Copy directory
	if err := copyDir(src, dst); err != nil {
		fmt.Printf("Error copying directory from %s to %s: %v\n", src, dst, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully copied %s to %s\n", src, dst)
}

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	// Gosec ignored: this is an internal maintenance script.
	in, err := os.Open(src) // #nosec G304
	if err != nil {
		return err
	}
	defer in.Close()

	// Gosec ignored: this is an internal maintenance script.
	out, err := os.Create(dst) // #nosec G304
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
