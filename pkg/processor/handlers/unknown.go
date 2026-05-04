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
	"context"
	"log"

	"github.com/tolusha/che-doc-generator/pkg/github"
)

func HandleUnknown(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)
}
