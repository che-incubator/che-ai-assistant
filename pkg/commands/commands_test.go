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
		{"/che-ai-assistant generate-che-doc", true, SubCommandGenerateCheDoc},
		{"/che-ai-assistant generate-che-doc\nsome text", true, SubCommandGenerateCheDoc},
		{"/che-ai-assistant help", true, SubCommandHelp},
		{"please /che-ai-assistant help thanks", true, SubCommandHelp},
		{"/che-ai-assistant   generate-che-doc    ", true, SubCommandGenerateCheDoc},
		{"\n   /che-ai-assistant generate-che-doc", true, SubCommandGenerateCheDoc},
		{"/che-ai-assistant", true, SubCommandHelp},
		{"/che-ai-assistant\n", true, SubCommandHelp},
		{"/che-ai-assistant  \n", true, SubCommandHelp},
		{"just a regular comment", false, ""},
		{"/che-ai-assistantly", false, ""},
		{"/generate-che-doc", false, ""},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprintf("Case #%d", i), func(t *testing.T) {
			ok, subCommandType := Parse(test.body)

			assert.Equal(t, test.isOk, ok)
			assert.Equal(t, test.expectedSubCommandType, subCommandType)
		})
	}
}
