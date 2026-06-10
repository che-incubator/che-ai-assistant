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

package scanner

import (
	"fmt"
	"testing"

	gh "github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
)

func TestCheckFiles(t *testing.T) {
	testCases := []struct {
		files    []string
		warnDirs []string
		expected []string
	}{
		{
			files:    []string{".claude/settings.json"},
			warnDirs: []string{".claude", ".vscode"},
			expected: []string{".claude/settings.json"},
		},
		{
			files:    []string{".vscode/launch.json", ".vscode/settings.json"},
			warnDirs: []string{".claude", ".vscode"},
			expected: []string{".vscode/launch.json", ".vscode/settings.json"},
		},
		{
			files:    []string{"src/main.go", "README.md"},
			warnDirs: []string{".claude", ".vscode"},
			expected: nil,
		},
		{
			files:    []string{"src/main.go", ".claude/config.yml", "README.md", ".vscode/tasks.json"},
			warnDirs: []string{".claude", ".vscode"},
			expected: []string{".claude/config.yml", ".vscode/tasks.json"},
		},
		{
			files:    []string{},
			warnDirs: []string{".claude", ".vscode"},
			expected: nil,
		},
		{
			files:    []string{".idea/workspace.xml"},
			warnDirs: []string{".idea"},
			expected: []string{".idea/workspace.xml"},
		},
		{
			files:    []string{".claudeignore", "not-vscode/file.txt"},
			warnDirs: []string{".claude", ".vscode"},
			expected: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Case #%d", i), func(t *testing.T) {
			files := make([]*gh.CommitFile, len(tc.files))
			for j, f := range tc.files {
				files[j] = &gh.CommitFile{Filename: gh.Ptr(f)}
			}

			result := CheckFiles(files, tc.warnDirs)

			assert.Equal(t, tc.expected, result)
		})
	}
}
