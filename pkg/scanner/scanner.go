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
	"strings"

	gh "github.com/google/go-github/v68/github"
)

func CheckFiles(files []*gh.CommitFile, warnDirs []string) []string {
	normalizedWarnDirs := make([]string, len(warnDirs))
	for i, warnDir := range warnDirs {
		if strings.HasSuffix(warnDir, "/") {
			normalizedWarnDirs[i] = warnDir
		} else {
			normalizedWarnDirs[i] = warnDir + "/"
		}
	}

	var matched []string
	for _, f := range files {
		fileName := f.GetFilename()

		for _, normalizedWarnDir := range normalizedWarnDirs {
			if strings.HasPrefix(fileName, normalizedWarnDir) {
				matched = append(matched, fileName)
				break
			}
		}
	}

	return matched
}
