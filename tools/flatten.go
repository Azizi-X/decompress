package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func rewriteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var results []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lineNoSpace := strings.ReplaceAll(line, " ", "")
		if !strings.HasPrefix(lineNoSpace, `#include"..`) {
			results = append(results, line)
		} else {
			parts := strings.Split(line, `"`)
			if len(parts) >= 2 {
				newPath := filepath.Base(parts[1])
				end := ""
				if len(parts) > 2 {
					end = strings.Join(parts[2:], `"`)
				}
				results = append(results, fmt.Sprintf(`#include "%s"%s`, newPath, end))
			} else {
				results = append(results, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for _, l := range results {
		_, err := outFile.WriteString(l + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	globPatterns := []string{"../*.c", "../*.h", "../*.S"}
	for _, pattern := range globPatterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Println("Error globbing:", err)
			continue
		}
		for _, file := range files {
			if err := rewriteFile(file); err != nil {
				fmt.Println("Error rewriting file:", err)
			}
		}
	}
}
