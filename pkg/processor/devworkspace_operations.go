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

package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
)

func (p *TaskProcessor) copyClaudeConfigInDevWorkspace(ctx context.Context, devWorkspaceName string) error {
	log.Printf("[INFO] Copying Claude config in the DevWorkspace %s", devWorkspaceName)

	err := p.devWorkspace.CopyClaudeConfig(ctx, devWorkspaceName)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to copy Claude config in the DevWorkspace %s", devWorkspaceName), err)
	}

	return nil
}
