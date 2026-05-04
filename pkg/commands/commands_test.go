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

package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		body                   string
		isOk                   bool
		expectedSubCommandType SubCommandType
	}{
		{"/generate-che-doc", true, SubCommandDefault},
		{"/generate-che-doc\nsome text", true, SubCommandDefault},
		{"/generate-che-doc help", true, SubCommandHelp},
		{"please /generate-che-doc help thanks", true, SubCommandHelp},
		{"/generate-che-doc   help    ", true, SubCommandHelp},
		{"\n   /generate-che-doc", true, SubCommandDefault},
		{"just a regular comment", false, ""},
		{"/generate-che-documentary", false, ""},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprintf("Case #%d", i), func(t *testing.T) {
			ok, subCommandType := Parse(test.body)

			assert.Equal(t, test.isOk, ok)
			assert.Equal(t, test.expectedSubCommandType, subCommandType)
		})
	}
}
