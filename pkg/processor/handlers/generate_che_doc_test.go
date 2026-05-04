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

package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDocPRURL(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		expected  string
		expectErr bool
	}{
		{
			name:     "URL in output",
			output:   `Some text\nhttps://github.com/eclipse-che/che-docs/pull/123\nDone`,
			expected: "https://github.com/eclipse-che/che-docs/pull/123",
		},
		{
			name:     "URL embedded in JSON",
			output:   `{"result": "https://github.com/eclipse-che/che-docs/pull/456"}`,
			expected: "https://github.com/eclipse-che/che-docs/pull/456",
		},
		{
			name:     "multiple URLs returns first",
			output:   "https://github.com/eclipse-che/che-docs/pull/1 and https://github.com/eclipse-che/che-docs/pull/2",
			expected: "https://github.com/eclipse-che/che-docs/pull/1",
		},
		{
			name:      "no URL in output",
			output:    "No PR was created",
			expectErr: true,
		},
		{
			name:      "wrong repo URL",
			output:    "https://github.com/other-org/other-repo/pull/99",
			expectErr: true,
		},
		{
			name:      "empty output",
			output:    "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDocPRURL(tt.output)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
