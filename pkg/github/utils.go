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

import "regexp"

var (
	githubRepository = regexp.MustCompile(`^(?:https?://[^/]+/)?([^/]+)/([^/]+?)(?:\.git)?$`)
)

func ParseRepoSlug(repo string) (owner, name string) {
	m := githubRepository.FindStringSubmatch(repo)
	if m == nil {
		return "", ""
	}

	return m[1], m[2]
}
