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

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStore_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)
	assert.NotNil(t, store)
	assert.False(t, store.IsProcessed("owner", "repo", 1, 100))
}

func TestNewStore_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	store, err := NewStore(path)
	require.NoError(t, err)
	assert.NotNil(t, store)
}

func TestNewStore_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	data := `{"entries":{"owner/repo#42":{"comment_ids":[111,222]}}}`
	err := os.WriteFile(path, []byte(data), 0644)
	require.NoError(t, err)

	store, err := NewStore(path)
	require.NoError(t, err)
	assert.True(t, store.IsProcessed("owner", "repo", 42, 111))
	assert.True(t, store.IsProcessed("owner", "repo", 42, 222))
	assert.False(t, store.IsProcessed("owner", "repo", 42, 333))
}

func TestMarkProcessed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)

	assert.False(t, store.IsProcessed("owner", "repo", 1, 100))

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)

	assert.True(t, store.IsProcessed("owner", "repo", 1, 100))

	// Verify file was written
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestMarkProcessed_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)

	// Should still only have one entry
	entry := store.entries["owner/repo#1"]
	assert.Len(t, entry.CommentIDs, 1)
}

func TestMarkProcessed_MultipleComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)
	err = store.MarkProcessed("owner", "repo", 1, 200)
	require.NoError(t, err)

	assert.True(t, store.IsProcessed("owner", "repo", 1, 100))
	assert.True(t, store.IsProcessed("owner", "repo", 1, 200))
	assert.Len(t, store.entries["owner/repo#1"].CommentIDs, 2)
}

func TestGetOpenKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)
	err = store.MarkProcessed("owner", "repo", 2, 200)
	require.NoError(t, err)

	keys := store.GetOpenKeys()
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "owner/repo#1")
	assert.Contains(t, keys, "owner/repo#2")
}

func TestRemoveKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)
	err = store.MarkProcessed("owner", "repo", 2, 200)
	require.NoError(t, err)

	err = store.RemoveKey("owner/repo#1")
	require.NoError(t, err)

	assert.False(t, store.IsProcessed("owner", "repo", 1, 100))
	assert.True(t, store.IsProcessed("owner", "repo", 2, 200))

	keys := store.GetOpenKeys()
	assert.Len(t, keys, 1)
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store1, err := NewStore(path)
	require.NoError(t, err)

	err = store1.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)

	// Load a new store from the same file
	store2, err := NewStore(path)
	require.NoError(t, err)

	assert.True(t, store2.IsProcessed("owner", "repo", 1, 100))
}

func TestParseKey(t *testing.T) {
	owner, repo, number, err := ParseKey("owner/repo#42")
	require.NoError(t, err)
	assert.Equal(t, "owner", owner)
	assert.Equal(t, "repo", repo)
	assert.Equal(t, 42, number)
}

func TestParseKey_Complex(t *testing.T) {
	owner, repo, number, err := ParseKey("che-incubator/che-docs#123")
	require.NoError(t, err)
	assert.Equal(t, "che-incubator", owner)
	assert.Equal(t, "che-docs", repo)
	assert.Equal(t, 123, number)
}

func TestParseKey_Invalid(t *testing.T) {
	_, _, _, err := ParseKey("invalid")
	assert.Error(t, err)

	_, _, _, err = ParseKey("owner/repo")
	assert.Error(t, err)

	_, _, _, err = ParseKey("owner/repo#abc")
	assert.Error(t, err)
}

func TestDifferentPRsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store, err := NewStore(path)
	require.NoError(t, err)

	err = store.MarkProcessed("owner", "repo", 1, 100)
	require.NoError(t, err)

	assert.True(t, store.IsProcessed("owner", "repo", 1, 100))
	assert.False(t, store.IsProcessed("owner", "repo", 2, 100))
	assert.False(t, store.IsProcessed("owner", "other-repo", 1, 100))
}

func TestNewStore_SetsStartTimeWhenFileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	before := time.Now().UTC()
	store, err := NewStore(path)
	after := time.Now().UTC()
	require.NoError(t, err)

	st := store.GetStartTime()
	require.NotNil(t, st)
	assert.False(t, st.Before(before))
	assert.False(t, st.After(after))

	_, err = os.Stat(path)
	require.NoError(t, err, "state file should be created on fresh start")
}

func TestNewStore_PersistsStartTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store1, err := NewStore(path)
	require.NoError(t, err)
	st1 := store1.GetStartTime()
	require.NotNil(t, st1)

	store2, err := NewStore(path)
	require.NoError(t, err)
	st2 := store2.GetStartTime()
	require.NotNil(t, st2)

	assert.True(t, st1.Equal(*st2), "start time should be preserved across loads")
}

func TestNewStore_ExistingFileWithStartTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	sd := stateData{
		StartTime: &original,
		Entries: map[string]*Entry{
			"owner/repo#1": {CommentIDs: []int64{100}},
		},
	}
	data, err := json.Marshal(sd)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	store, err := NewStore(path)
	require.NoError(t, err)

	st := store.GetStartTime()
	require.NotNil(t, st)
	assert.True(t, st.Equal(original), "should preserve original start time")
	assert.True(t, store.IsProcessed("owner", "repo", 1, 100))
}

func TestNewStore_ExistingFileWithoutStartTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	data := `{"entries":{"owner/repo#1":{"comment_ids":[100]}}}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	store, err := NewStore(path)
	require.NoError(t, err)

	assert.Nil(t, store.GetStartTime(), "should not set start time for existing file without one")
	assert.True(t, store.IsProcessed("owner", "repo", 1, 100))
}
