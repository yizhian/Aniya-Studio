package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadDotEnvFromFile loads KEY=VALUE pairs from a .env-style file into process env.
// Existing environment variables are kept (file values do not override explicit exports).
func loadDotEnvFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			return fmt.Errorf(".env parse error at line %d", lineNo)
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			return fmt.Errorf(".env parse error at line %d: empty key", lineNo)
		}

		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		// Keep explicitly exported env values as highest priority.
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
