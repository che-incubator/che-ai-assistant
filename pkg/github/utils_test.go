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

package github

import "testing"

func TestParseRepoSlug(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantName  string
	}{
		{
			name:      "owner/repo slug",
			input:     "eclipse-che/che",
			wantOwner: "eclipse-che",
			wantName:  "che",
		},
		{
			name:      "HTTPS URL",
			input:     "https://github.com/eclipse-che/che",
			wantOwner: "eclipse-che",
			wantName:  "che",
		},
		{
			name:      "HTTPS URL with .git suffix",
			input:     "https://github.com/eclipse-che/che.git",
			wantOwner: "eclipse-che",
			wantName:  "che",
		},
		{
			name:      "HTTP URL",
			input:     "http://github.com/eclipse-che/che",
			wantOwner: "eclipse-che",
			wantName:  "che",
		},
		{
			name:      "empty string",
			input:     "",
			wantOwner: "",
			wantName:  "",
		},
		{
			name:      "single segment",
			input:     "che",
			wantOwner: "",
			wantName:  "",
		},
		{
			name:      "trailing slash",
			input:     "eclipse-che/che/",
			wantOwner: "",
			wantName:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, name := ParseRepoSlug(tt.input)
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}
