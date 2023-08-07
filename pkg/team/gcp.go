package team

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"cloud.google.com/go/iam"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/k8s"
	"golang.org/x/exp/slices"
	"google.golang.org/api/googleapi"
	iamv1 "google.golang.org/api/iam/v1"
	"google.golang.org/grpc/codes"
)

const secretRoleName = "roles/owner"

func (c Client) createGCPTeamResources(ctx context.Context, team gensql.Team) error {
	if c.dryRun {
		return nil
	}

	sa, err := c.createIAMServiceAccount(ctx, team.ID)
	if err != nil {
		return err
	}

	secret, err := c.createSecret(ctx, team.Slug, team.ID)
	if err != nil {
		return err
	}

	if err := c.createServiceAccountSecretAccessorBinding(ctx, sa.Email, secret.Name); err != nil {
		return err
	}

	if err := c.setUsersSecretOwnerBinding(ctx, team.Users, secret.Name); err != nil {
		return err
	}

	if err := c.createSAWorkloadIdentityBinding(ctx, sa.Email, team.ID); err != nil {
		return err
	}

	return nil
}

func (c Client) createSAWorkloadIdentityBinding(ctx context.Context, email, teamID string) error {
	service, err := iamv1.NewService(ctx)
	if err != nil {
		return err
	}

	resource := fmt.Sprintf("projects/%v/serviceAccounts/%v", c.gcpProject, email)

	policy, err := service.Projects.ServiceAccounts.GetIamPolicy(resource).Do()
	if err != nil {
		return err
	}

	namespace := k8s.TeamIDToNamespace(teamID)
	bindings := policy.Bindings
	if !c.updateRoleBindingIfExists(bindings, "roles/iam.workloadIdentityUser", namespace, teamID) {
		bindings = append(bindings, &iamv1.Binding{
			Members: []string{fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", c.gcpProject, namespace, teamID)},
			Role:    "roles/iam.workloadIdentityUser",
		})
	}

	_, err = service.Projects.ServiceAccounts.SetIamPolicy(resource, &iamv1.SetIamPolicyRequest{
		Policy: &iamv1.Policy{
			Bindings: bindings,
		},
	}).Do()
	if err != nil {
		return err
	}

	return nil
}

func (c Client) updateRoleBindingIfExists(bindings []*iamv1.Binding, role, namespace, team string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			binding.Members = append(binding.Members, fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", c.gcpProject, namespace, team))
			return true
		}
	}

	return false
}

func (c Client) createSecret(ctx context.Context, slug, teamID string) (*secretmanagerpb.Secret, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/" + c.gcpProject,
		SecretId: teamID,
		Secret: &secretmanagerpb.Secret{
			Labels: map[string]string{
				"team":       slug,
				"created-by": "knorten",
			},
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_UserManaged_{
					UserManaged: &secretmanagerpb.Replication_UserManaged{
						Replicas: []*secretmanagerpb.Replication_UserManaged_Replica{
							{
								Location: c.gcpRegion,
							},
						},
					},
				},
			},
		},
	}

	s, err := client.CreateSecret(ctx, req)
	if err != nil {
		apiError, ok := apierror.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.AlreadyExists {
				return client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
					Name: fmt.Sprintf("projects/%v/secrets/%v", c.gcpProject, teamID),
				})
			}
		}
		return nil, err
	}

	return s, nil
}

func (c Client) createServiceAccountSecretAccessorBinding(ctx context.Context, sa, secret string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	handle := client.IAM(secret)
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	policy.Add("serviceAccount:"+sa, "roles/secretmanager.secretAccessor")
	if err = handle.SetPolicy(ctx, policy); err != nil {
		return err
	}

	return nil
}

func (c Client) createIAMServiceAccount(ctx context.Context, team string) (*iamv1.ServiceAccount, error) {
	service, err := iamv1.NewService(ctx)
	if err != nil {
		return nil, err
	}

	request := &iamv1.CreateServiceAccountRequest{
		AccountId: team,
		ServiceAccount: &iamv1.ServiceAccount{
			DisplayName: fmt.Sprintf("Service account for team %v", team),
		},
	}

	account, err := service.Projects.ServiceAccounts.Create("projects/"+c.gcpProject, request).Do()
	if err != nil {
		var gError *googleapi.Error
		ok := errors.As(err, &gError)
		if ok {
			if gError.Code == http.StatusConflict {
				serviceAccountName := fmt.Sprintf("projects/%v/serviceAccounts/%v@%v.iam.gserviceaccount.com", c.gcpProject, team, c.gcpProject)
				return service.Projects.ServiceAccounts.Get(serviceAccountName).Do()
			}
		}

		return nil, err
	}

	return account, nil
}

func (c Client) updateGCPTeamResources(ctx context.Context, team gensql.Team) error {
	if c.dryRun {
		return nil
	}

	return c.setUsersSecretOwnerBinding(ctx, team.Users, fmt.Sprintf("projects/%v/secrets/%v", c.gcpProject, team.ID))
}

func (c Client) deleteGCPTeamResources(ctx context.Context, teamID string) error {
	if c.dryRun {
		return nil
	}

	if err := c.deleteIAMServiceAccount(ctx, teamID); err != nil {
		return err
	}

	if err := c.deleteSecret(ctx, teamID); err != nil {
		return err
	}

	return nil
}

func (c Client) deleteSecret(ctx context.Context, teamID string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	project := fmt.Sprintf("projects/%v", c.gcpProject)
	_ = client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent:   project,
		PageSize: int32(500),
	})

	req := &secretmanagerpb.DeleteSecretRequest{
		Name: fmt.Sprintf("%v/secrets/%v", project, teamID),
	}

	err = client.DeleteSecret(ctx, req)
	if err != nil {
		apiError, ok := apierror.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.NotFound {
				return nil
			}
		}

		return err
	}

	return nil
}

func (c Client) deleteIAMServiceAccount(ctx context.Context, teamID string) error {
	service, err := iamv1.NewService(ctx)
	if err != nil {
		return err
	}

	sa := fmt.Sprintf("projects/%v/serviceAccounts/%v@%v.iam.gserviceaccount.com", c.gcpProject, teamID, c.gcpProject)
	_, err = service.Projects.ServiceAccounts.Delete(sa).Do()
	if err != nil {
		apiError, ok := err.(*googleapi.Error)
		if ok && apiError.Code == http.StatusNotFound {
			return nil
		}

		return err
	}

	return nil
}

func (c Client) setUsersSecretOwnerBinding(ctx context.Context, users []string, secret string) error {
	users = addUserTypePrefix(users)

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

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

	err = handle.SetPolicy(ctx, policy)
	if err != nil {
		return err
	}

	for _, user := range users {
		if err := c.updatePolicy(ctx, handle, user); err != nil {
			return err
		}
	}

	return nil
}

func (c Client) updatePolicy(ctx context.Context, handle *iam.Handle, user string) error {
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	policyMembers := policy.Members(secretRoleName)
	if !slices.Contains(policyMembers, user) {
		policy.Add(user, secretRoleName)
		err = handle.SetPolicy(ctx, policy)
		if err != nil {
			apiError, ok := apierror.FromError(err)
			if ok && apiError.GRPCStatus().Code() == codes.InvalidArgument {
				return nil

			}

			return err
		}
	}

	return nil
}

func addUserTypePrefix(users []string) []string {
	prefixedUsers := make([]string, len(users))
	for i, u := range users {
		prefixedUsers[i] = "user:" + u
	}

	return prefixedUsers
}
