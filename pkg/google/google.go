package google

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iam/v1"
)

type Google struct {
	DryRun bool
}

func New(dryRun bool) *Google {
	return &Google{
		DryRun: dryRun,
	}
}

func (g *Google) CreateIAMServiceAccount(ctx context.Context, parent, team string) (*iam.ServiceAccount, error) {
	service, err := iam.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("iam.NewService: %v", err)
	}

	request := &iam.CreateServiceAccountRequest{
		AccountId: team,
		ServiceAccount: &iam.ServiceAccount{
			DisplayName: fmt.Sprintf("Service account for team %v", team),
		},
	}

	account, err := service.Projects.ServiceAccounts.Create(parent, request).Do()
	if err != nil {
		return nil, fmt.Errorf("Projects.ServiceAccounts.Create: %v", err)
	}

	return account, nil
}

func (g *Google) CreateGSMSecret(ctx context.Context, parent, team string) (*secretmanagerpb.Secret, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer client.Close()

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: team,
		Secret: &secretmanagerpb.Secret{
			Labels: map[string]string{"team": team},
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

	result, err := client.CreateSecret(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret: %v", err)
	}

	return result, nil
}

func (g *Google) CreateSASecretAccessorBinding(ctx context.Context, sa, secret string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer client.Close()

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

func (g *Google) CreateUserSecretOwnerBindings(ctx context.Context, users []string, secret string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer client.Close()

	handle := client.IAM(secret)
	policy, err := handle.Policy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get policy: %v", err)
	}

	for _, user := range users {
		policy.Add("user:"+user, "roles/owner")
	}

	if err = handle.SetPolicy(ctx, policy); err != nil {
		return fmt.Errorf("failed to save policy: %v", err)
	}
	return nil
}

func (g *Google) CreateSAWorkloadIdentityBinding(ctx context.Context, iamSA, namespace string) error {
	service, err := iam.NewService(ctx)
	if err != nil {
		return err
	}

	resource := "projects/knada-gcp/serviceAccounts/" + iamSA

	policy, err := service.Projects.ServiceAccounts.GetIamPolicy(resource).Do()
	if err != nil {
		return err
	}
	bindings := policy.Bindings
	for _, b := range bindings {
		if b.Role == "roles/iam.workloadIdentityUser" {
			b.Members = append(b.Members, fmt.Sprintf("serviceAccount:knada-gcp.svc.id.goog[%v/default]", namespace))
		}
	}

	_, err = service.Projects.ServiceAccounts.SetIamPolicy(resource, &iam.SetIamPolicyRequest{
		Policy: &iam.Policy{
			Bindings: bindings,
		},
	}).Do()
	if err != nil {
		return err
	}

	return nil
}
