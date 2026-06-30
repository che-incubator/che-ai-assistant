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

func TestParseUpdateCheE2ETestsPRURL(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		expected  string
		expectErr bool
	}{
		{
			name:     "PR URL in output",
			output:   "Some text\nhttps://github.com/eclipse-che/che/pull/123\nDone",
			expected: "https://github.com/eclipse-che/che/pull/123",
		},
		{
			name:     "PR URL embedded in JSON",
			output:   `{"result": "https://github.com/eclipse-che/che/pull/456"}`,
			expected: "https://github.com/eclipse-che/che/pull/456",
		},
		{
			name:     "multiple PR URLs returns first",
			output:   "https://github.com/eclipse-che/che/pull/10 and https://github.com/eclipse-che/che/pull/20",
			expected: "https://github.com/eclipse-che/che/pull/10",
		},
		{
			name:      "no URL in output",
			output:    "No PR was created",
			expectErr: true,
		},
		{
			name:      "wrong repo URL",
			output:    "https://github.com/other-org/other-repo/pull/123",
			expectErr: true,
		},
		{
			name:      "che-dashboard repo URL does not match",
			output:    "https://github.com/eclipse-che/che-dashboard/pull/123",
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
			result, err := parseUpdateCheE2ETestsPRURL(tt.output)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
