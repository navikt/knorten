package google

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/api/iam/v1"
)

const (
	secretRoleName = "roles/owner"
)

type Google struct {
	dryRun  bool
	log     *logrus.Entry
	project string
	region  string
}

func New(log *logrus.Entry, gcpProject, gcpRegion string, dryRun bool) *Google {
	return &Google{
		log:     log,
		project: gcpProject,
		region:  gcpRegion,
		dryRun:  dryRun,
	}
}

func (g *Google) createIAMServiceAccount(ctx context.Context, team string) (*iam.ServiceAccount, error) {
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

	account, err := service.Projects.ServiceAccounts.Create("projects/"+g.project, request).Do()
	if err != nil {
		return nil, fmt.Errorf("Projects.ServiceAccounts.Create: %v", err)
	}

	return account, nil
}

func (g *Google) createSAWorkloadIdentityBinding(ctx context.Context, iamSA, team string) error {
	service, err := iam.NewService(ctx)
	if err != nil {
		return err
	}

	resource := fmt.Sprintf("projects/%v/serviceAccounts/%v", g.project, iamSA)

	policy, err := service.Projects.ServiceAccounts.GetIamPolicy(resource).Do()
	if err != nil {
		return err
	}
	bindings := policy.Bindings
	if !g.updateRoleBindingIfExists(bindings, "roles/iam.workloadIdentityUser", team) {
		// Create role binding if not exists
		bindings = append(bindings, &iam.Binding{
			Members: []string{fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", g.project, team, team)},
			Role:    "roles/iam.workloadIdentityUser",
		})
	}

	for _, b := range bindings {
		if b.Role == "roles/iam.workloadIdentityUser" {
			b.Members = append(b.Members, fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", g.project, team, team))
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

func (g *Google) updateRoleBindingIfExists(bindings []*iam.Binding, role, team string) bool {
	for _, b := range bindings {
		if b.Role == role {
			b.Members = append(b.Members, fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", g.project, team, team))
			return true
		}
	}
	return false
}

func (g *Google) CreateGCPResources(c context.Context, team string, users []string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	iamSA, err := g.createIAMServiceAccount(c, team)
	if err != nil {
		return err
	}

	gsmSecret, err := g.createSecret(c, team)
	if err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}

	if err := g.createServiceAccountSecretAccessorBinding(c, iamSA.Email, gsmSecret.Name); err != nil {
		return err
	}

	if err := g.setUsersSecretOwnerBinding(c, users, gsmSecret.Name); err != nil {
		return fmt.Errorf("failed while creating secret binding: %v", err)
	}

	if err := g.createSAWorkloadIdentityBinding(c, iamSA.Email, team); err != nil {
		return err
	}

	return nil
}

func (g *Google) Update(c context.Context, secret string, users []string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	return g.setUsersSecretOwnerBinding(c, users, fmt.Sprintf("projects/%v/secrets/%v", g.project, secret))
}
