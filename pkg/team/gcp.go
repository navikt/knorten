package team

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/gcp"
	"github.com/nais/knorten/pkg/k8s"
	"google.golang.org/api/googleapi"
	iamv1 "google.golang.org/api/iam/v1"
)

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

	if err := gcp.SetUsersSecretOwnerBinding(ctx, team.Users, secret.Name); err != nil {
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
	return gcp.CreateSecret(ctx, c.gcpProject, c.gcpRegion, teamID, map[string]string{"team": slug})
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

	if err := c.createServiceAccountSecretAccessorBinding(ctx, fmt.Sprintf("%v@%v.iam.gserviceaccount.com", team.ID, c.gcpProject), team.ID); err != nil {
		return err
	}

	return gcp.SetUsersSecretOwnerBinding(ctx, team.Users, fmt.Sprintf("projects/%v/secrets/%v", c.gcpProject, team.ID))
}

func (c Client) deleteGCPTeamResources(ctx context.Context, teamID string) error {
	if c.dryRun {
		return nil
	}

	if err := c.deleteIAMServiceAccount(ctx, teamID); err != nil {
		return err
	}

	if err := gcp.DeleteSecret(ctx, c.gcpProject, teamID); err != nil {
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
		var apiError *googleapi.Error
		ok := errors.As(err, &apiError)
		if ok && apiError.Code == http.StatusNotFound {
			return nil
		}

		return err
	}

	return nil
}
