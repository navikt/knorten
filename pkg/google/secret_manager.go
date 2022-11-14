package google

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"context"
	"fmt"
	"k8s.io/utils/strings/slices"
)

func (g *Google) closeClientFunc() func(client *secretmanager.Client) {
	return func(client *secretmanager.Client) {
		err := client.Close()
		if err != nil {
			g.log.WithError(err).Error("failed closing client")
		}
	}
}

func (g *Google) createSecret(ctx context.Context, team string) (*secretmanagerpb.Secret, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer g.closeClientFunc()(client)

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   g.project,
		SecretId: team,
		Secret: &secretmanagerpb.Secret{
			Labels: map[string]string{
				"team":                team,
				"knada.io/created-by": "knorten",
			},
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_UserManaged_{
					UserManaged: &secretmanagerpb.Replication_UserManaged{
						Replicas: []*secretmanagerpb.Replication_UserManaged_Replica{
							{
								Location: "europe-west1",
							},
						},
					},
				},
			},
		},
	}

	return client.CreateSecret(ctx, req)
}

func (g *Google) createServiceAccountSecretAccessorBinding(ctx context.Context, sa, secret string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer g.closeClientFunc()(client)

	handle := client.IAM(secret)
	policy, err := handle.Policy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get policy: %v", err)
	}

	policy.Add("serviceAccount:"+sa, "roles/secretmanager.secretAccessor")
	if err = handle.SetPolicy(ctx, policy); err != nil {
		return fmt.Errorf("failed to save policy: %v", err)
	}

	return nil
}

func (g *Google) setUsersSecretOwnerBinding(ctx context.Context, users []string, secret string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer g.closeClientFunc()(client)

	handle := client.IAM(secret)
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	policyMembers := policy.Members(secretRoleName)

	for _, member := range policyMembers {
		if !slices.Contains(users, member) {
			policy.Remove(member, secretRoleName)
		}
	}

	for _, user := range users {
		if !slices.Contains(policyMembers, user) {
			policy.Add("user:"+user, secretRoleName)
		}
	}

	return handle.SetPolicy(ctx, policy)
}
