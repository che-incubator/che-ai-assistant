//
// Copyright (c) 2026 Red Hat, Inc.
// Licensed under the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

// Package state manages a JSON file that tracks which trigger comments have
// been processed. Example state.json:
//
//	{
//	  "che-incubator/che-docs#42": {
//	    "comment_ids": [123456789, 987654321]
//	  },
//	  "eclipse-che/che-server#100": {
//	    "comment_ids": [555666777]
//	  }
//	}
package state

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Entry struct {
	CommentIDs []int64 `json:"comment_ids"`
}

type Store struct {
	mu       sync.Mutex
	filePath string
	entries  map[string]*Entry
}

func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		entries:  make(map[string]*Entry),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) IsProcessed(owner, repo string, number int, commentID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := makeKey(owner, repo, number)
	entry, ok := s.entries[key]
	if !ok {
		return false
	}

	for _, id := range entry.CommentIDs {
		if id == commentID {
			return true
		}
	}

	return false
}

func (s *Store) MarkProcessed(owner, repo string, number int, commentID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := makeKey(owner, repo, number)
	entry, ok := s.entries[key]
	if !ok {
		entry = &Entry{}
		s.entries[key] = entry
	}

	for _, id := range entry.CommentIDs {
		if id == commentID {
			return nil
		}
	}

	entry.CommentIDs = append(entry.CommentIDs, commentID)

	return s.save()
}

func (s *Store) GetOpenKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}

	return keys
}

func (s *Store) RemoveKey(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)

	return s.save()
}

func ParseKey(key string) (owner, repo string, number int, err error) {
	slashIdx := strings.Index(key, "/")
	if slashIdx < 1 {
		return "", "", 0, fmt.Errorf("invalid key format %q", key)
	}

	hashIdx := strings.Index(key[slashIdx+1:], "#")
	if hashIdx < 1 {
		return "", "", 0, fmt.Errorf("invalid key format %q", key)
	}
	hashIdx += slashIdx + 1

	owner = key[:slashIdx]
	repo = key[slashIdx+1 : hashIdx]
	number, err = strconv.Atoi(key[hashIdx+1:])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid key format %q: %w", key, err)
	}

	return owner, repo, number, nil
}

func makeKey(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading state file: %w", err)
	}

	if len(data) == 0 {
		s.entries = make(map[string]*Entry)
		return nil
	}

	return json.Unmarshal(data, &s.entries)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	tmp, err := os.CreateTemp(dir, "state-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		if cerr := tmp.Close(); cerr != nil {
			log.Printf("[WARN] Failed to close temp file %s: %v", tmp.Name(), cerr)
		}
		if rerr := os.Remove(tmp.Name()); rerr != nil {
			log.Printf("[WARN] Failed to remove temp file %s: %v", tmp.Name(), rerr)
		}
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		if rerr := os.Remove(tmp.Name()); rerr != nil {
			log.Printf("[WARN] Failed to remove temp file %s: %v", tmp.Name(), rerr)
		}
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmp.Name(), s.filePath); err != nil {
		if rerr := os.Remove(tmp.Name()); rerr != nil {
			log.Printf("[WARN] Failed to remove temp file %s: %v", tmp.Name(), rerr)
		}
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
