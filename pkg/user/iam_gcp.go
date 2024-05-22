package user

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
)

var gcpIAMPolicyBindingsRoles = []string{
	"roles/compute.viewer",
	"roles/iap.tunnelResourceAccessor",
	"roles/monitoring.viewer",
}

func (c Client) createIAMPolicyBindingsInGCP(ctx context.Context, instanceName, email string) error {
	if c.dryRun {
		return nil
	}

	if err := c.addComputeInstanceOwnerBindingInGCP(ctx, instanceName, email); err != nil {
		return err
	}

	if err := c.addOpsServiceAccountUserBinding(ctx, email); err != nil {
		return err
	}

	for _, role := range gcpIAMPolicyBindingsRoles {
		if err := c.addProjectIAMPolicyBindingInGCP(ctx, email, role); err != nil {
			return err
		}
	}

	return nil
}

func (c Client) deleteIAMPolicyBindingsFromGCP(ctx context.Context, email string) error {
	if c.dryRun {
		return nil
	}

	if err := c.removeOpsServiceAccountUserBinding(ctx, email); err != nil {
		return err
	}

	for _, role := range gcpIAMPolicyBindingsRoles {
		if err := c.removeProjectIAMPolicyBindingFromGCP(ctx, email, role); err != nil {
			return err
		}
	}

	return nil
}

func (c Client) addComputeInstanceOwnerBindingInGCP(ctx context.Context, instanceName, user string) error {
	role := "roles/owner"
	computeClient, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return err
	}
	defer computeClient.Close()

	req := &computepb.GetIamPolicyInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Resource: instanceName,
	}
	policy, err := computeClient.GetIamPolicy(ctx, req)
	if err != nil {
		return err
	}

	policy = addComputePolicyBindingMember(policy, role, user)
	setReq := &computepb.SetIamPolicyInstanceRequest{
		Project:  c.gcpProject,
		Zone:     c.gcpZone,
		Resource: instanceName,
		ZoneSetPolicyRequestResource: &computepb.ZoneSetPolicyRequest{
			Policy: policy,
		},
	}

	_, err = computeClient.SetIamPolicy(ctx, setReq)
	if err != nil {
		return err
	}

	return nil
}

func (c Client) addOpsServiceAccountUserBinding(ctx context.Context, email string) error {
	iamService, err := iam.NewService(ctx)
	if err != nil {
		return err
	}

	userWithType := fmt.Sprintf("user:%v", email)

	resp, err := iamService.Projects.ServiceAccounts.GetIamPolicy(c.opsAgentSAResource).Context(ctx).Do()
	if err != nil {
		return err
	}

	bindings := addServiceAccountUserBinding(resp.Bindings, "roles/iam.serviceAccountUser", userWithType)
	_, err = iamService.Projects.ServiceAccounts.SetIamPolicy(c.opsAgentSAResource, &iam.SetIamPolicyRequest{
		Policy: &iam.Policy{
			Bindings: bindings,
		},
	}).Do()
	if err != nil {
		return err
	}
	return nil
}

func (c Client) removeOpsServiceAccountUserBinding(ctx context.Context, email string) error {
	iamService, err := iam.NewService(ctx)
	if err != nil {
		return err
	}
	userWithType := fmt.Sprintf("user:%v", email)

	resp, err := iamService.Projects.ServiceAccounts.GetIamPolicy(c.opsAgentSAResource).Context(ctx).Do()
	if err != nil {
		return err
	}

	bindings := removeServiceAccountUserBinding(resp.Bindings, "roles/iam.serviceAccountUser", userWithType)
	_, err = iamService.Projects.ServiceAccounts.SetIamPolicy(c.opsAgentSAResource, &iam.SetIamPolicyRequest{
		Policy: &iam.Policy{
			Bindings: bindings,
		},
	}).Do()
	if err != nil {
		return err
	}

	return nil
}

func (c Client) addProjectIAMPolicyBindingInGCP(ctx context.Context, user, role string) error {
	client, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return err
	}

	policy, err := client.Projects.GetIamPolicy(c.gcpProject, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		return err
	}
	policy.Bindings = addProjectRoleBindingForUser(policy.Bindings, role, fmt.Sprintf("user:%v", user))

	_, err = client.Projects.SetIamPolicy(c.gcpProject, &cloudresourcemanager.SetIamPolicyRequest{
		Policy: policy,
	}).Do()
	if err != nil {
		return err
	}

	return nil
}

func (c Client) removeProjectIAMPolicyBindingFromGCP(ctx context.Context, user, role string) error {
	client, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return err
	}

	policy, err := client.Projects.GetIamPolicy(c.gcpProject, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		return err
	}
	policy.Bindings = removeProjectRoleBindingForUser(policy.Bindings, role, fmt.Sprintf("user:%v", user))

	_, err = client.Projects.SetIamPolicy(c.gcpProject, &cloudresourcemanager.SetIamPolicyRequest{
		Policy: policy,
	}).Do()
	if err != nil {
		return err
	}

	return nil
}

func addComputePolicyBindingMember(policy *computepb.Policy, role, email string) *computepb.Policy {
	for _, binding := range policy.Bindings {
		if binding.Role != nil && *binding.Role == role {
			binding.Members = append(binding.Members, fmt.Sprintf("user:%v", email))
			return policy
		}
	}

	policy.Bindings = append(policy.Bindings, &computepb.Binding{
		Members: []string{fmt.Sprintf("user:%v", email)},
		Role:    &role,
	})

	return policy
}

func addServiceAccountUserBinding(bindings []*iam.Binding, role, user string) []*iam.Binding {
	for _, b := range bindings {
		if b.Role == role {
			b.Members = append(b.Members, user)
			return bindings
		}
	}

	return append(bindings, &iam.Binding{
		Members: []string{user},
		Role:    role,
	})
}

func addProjectRoleBindingForUser(bindings []*cloudresourcemanager.Binding, role, user string) []*cloudresourcemanager.Binding {
	for _, b := range bindings {
		if b.Role == role {
			b.Members = append(b.Members, user)
		}
	}

	return bindings
}

func removeServiceAccountUserBinding(bindings []*iam.Binding, role, user string) []*iam.Binding {
	for _, b := range bindings {
		if b.Role == role {
			b.Members = removeMember(b.Members, user)
		}
	}

	return bindings
}

func removeProjectRoleBindingForUser(bindings []*cloudresourcemanager.Binding, role, user string) []*cloudresourcemanager.Binding {
	for _, b := range bindings {
		if b.Role == role {
			b.Members = removeMember(b.Members, user)
		}
	}

	return bindings
}

func removeMember(members []string, user string) []string {
	for idx, member := range members {
		if member == user {
			return append(members[:idx], members[idx+1:]...)
		}
	}

	return members
}
