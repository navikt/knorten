package user

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/gcp"
)

func (c Client) createUserGSMInGCP(ctx context.Context, name, owner string) error {
	if c.dryRun {
		return nil
	}

	secret, err := gcp.CreateSecret(ctx, c.gcpProject, c.gcpRegion, name, map[string]string{"owner": name})
	if err != nil {
		return err
	}

	if err := gcp.SetUsersSecretOwnerBinding(ctx, []string{owner}, secret.Name); err != nil {
		return err
	}

	return nil
}

func (c Client) deleteUserGSMFromGCP(ctx context.Context, name string) error {
	if c.dryRun {
		return nil
	}

	if err := gcp.DeleteSecret(ctx, fmt.Sprintf("projects/%v/secrets/%v", c.gcpProject, name)); err != nil {
		return err
	}

	return nil
}
