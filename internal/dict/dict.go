// Package dict loads a word list and provides case-insensitive membership
// lookup. The default source is the ENABLE word list (public domain, 172,820
// words). TWL/Collins are copyrighted; swap the file to substitute them.
package dict

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

// Dictionary is an in-memory set of uppercase words.
type Dictionary struct {
	words map[string]struct{}
}

// Contains reports whether the word (any case) is in the dictionary.
func (d *Dictionary) Contains(word string) bool {
	if d == nil {
		return false
	}
	_, ok := d.words[strings.ToUpper(word)]
	return ok
}

// Size returns the number of words loaded.
func (d *Dictionary) Size() int {
	if d == nil {
		return 0
	}
	return len(d.words)
}

// LoadFile loads a dictionary from a plain-text or gzipped file (one word per
// line). Files with a ".gz" extension are decompressed transparently.
func LoadFile(path string) (*Dictionary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("gzip %s: %w", path, err)
		}
		defer gz.Close()
		r = gz
	}
	return LoadReader(r)
}

// LoadReader loads a dictionary from an io.Reader of newline-separated words.
func LoadReader(r io.Reader) (*Dictionary, error) {
	d := &Dictionary{words: make(map[string]struct{}, 200_000)}
	scanner := bufio.NewScanner(r)
	// Allow longer lines defensively; default 64KiB is fine for word lists.
	scanner.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for scanner.Scan() {
		w := strings.TrimSpace(scanner.Text())
		if w == "" {
			continue
		}
		d.words[strings.ToUpper(w)] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan dictionary: %w", err)
	}
	return d, nil
}

// FromWords returns a dictionary populated from the given slice (useful in
// tests).
func FromWords(words []string) *Dictionary {
	d := &Dictionary{words: make(map[string]struct{}, len(words))}
	for _, w := range words {
		d.words[strings.ToUpper(w)] = struct{}{}
	}
	return d
}
