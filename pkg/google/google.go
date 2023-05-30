package google

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
)

const (
	secretRoleName = "roles/owner"
)

type Google struct {
	dryRun          bool
	log             *logrus.Entry
	repo            *database.Repo
	project         string
	region          string
	vmNetworkConfig string
}

func New(log *logrus.Entry, repo *database.Repo, gcpProject, gcpRegion, vmNetworkConfig string, dryRun bool) *Google {
	return &Google{
		log:             log,
		repo:            repo,
		project:         gcpProject,
		region:          gcpRegion,
		vmNetworkConfig: vmNetworkConfig,
		dryRun:          dryRun,
	}
}

func (g *Google) CreateGCPTeamResources(c context.Context, slug, teamID string, users []string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	iamSA, err := g.createIAMServiceAccount(c, teamID)
	if err != nil {
		return err
	}

	gsmSecret, err := g.createSecret(c, slug, teamID)
	if err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}

	if err := g.createServiceAccountSecretAccessorBinding(c, iamSA.Email, gsmSecret.Name); err != nil {
		return err
	}

	if err := g.setUsersSecretOwnerBinding(c, users, gsmSecret.Name); err != nil {
		return fmt.Errorf("failed while creating secret binding: %v", err)
	}

	if err := g.createSAWorkloadIdentityBinding(c, iamSA.Email, k8s.NameToNamespace(teamID), teamID); err != nil {
		return err
	}

	return nil
}

func (g *Google) Update(c context.Context, teamSlug string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	team, err := g.repo.TeamGet(c, teamSlug)
	if err != nil {
		return err
	}

	instance, err := g.repo.ComputeInstanceGet(c, team.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if err := g.UpdateComputeInstanceOwners(c, instance.InstanceName, team.Slug); err != nil {
		return err
	}

	return g.setUsersSecretOwnerBinding(c, team.Users, fmt.Sprintf("projects/%v/secrets/%v", g.project, team.ID))
}

func (g *Google) DeleteGCPTeamResources(c context.Context, team gensql.TeamGetRow, instance gensql.ComputeInstance) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	if err := g.deleteIAMServiceAccount(c, team.ID); err != nil {
		g.log.WithError(err).Errorf("deleting iam service account %v", team.ID)
		return err
	}

	if err := g.deleteSecret(c, team.ID); err != nil {
		g.log.WithError(err).Errorf("deleting gsm secret %v", team.ID)
		return err
	}

	if instance.InstanceName != "" {
		if err := g.deleteComputeInstance(c, instance.InstanceName, team.Users); err != nil {
			g.log.WithError(err).Errorf("deleting compute instance %v", instance.InstanceName)
			return err
		}
	}

	return nil
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
		gError, ok := err.(*googleapi.Error)
		if ok {
			if gError.Code == 409 {
				g.log.Infof("create iam service account: service account %v already exists", team)
				return g.getIAMServiceAccount(service, team)
			}
		}
		return nil, fmt.Errorf("Projects.ServiceAccounts.Create: %v", err)
	}

	return account, nil
}

func (g *Google) getIAMServiceAccount(service *iam.Service, saName string) (*iam.ServiceAccount, error) {
	sa := fmt.Sprintf("projects/%v/serviceAccounts/%v@%v.iam.gserviceaccount.com", g.project, saName, g.project)
	return service.Projects.ServiceAccounts.Get(sa).Do()
}

func (g *Google) createSAWorkloadIdentityBinding(ctx context.Context, iamSA, k8sNamespace, team string) error {
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
	if !g.updateRoleBindingIfExists(bindings, "roles/iam.workloadIdentityUser", k8sNamespace, team) {
		// Add role binding if not exists
		bindings = append(bindings, &iam.Binding{
			Members: []string{fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", g.project, k8sNamespace, team)},
			Role:    "roles/iam.workloadIdentityUser",
		})
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

func (g *Google) updateRoleBindingIfExists(bindings []*iam.Binding, role, k8sNamespace, team string) bool {
	for _, b := range bindings {
		if b.Role == role {
			b.Members = append(b.Members, fmt.Sprintf("serviceAccount:%v.svc.id.goog[%v/%v]", g.project, k8sNamespace, team))
			return true
		}
	}
	return false
}

func (g *Google) deleteIAMServiceAccount(ctx context.Context, teamID string) error {
	service, err := iam.NewService(ctx)
	if err != nil {
		return fmt.Errorf("iam.NewService: %v", err)
	}

	sa := fmt.Sprintf("projects/%v/serviceAccounts/%v@%v.iam.gserviceaccount.com", g.project, teamID, g.project)
	_, err = service.Projects.ServiceAccounts.Delete(sa).Do()
	if err != nil {
		apiError, ok := err.(*googleapi.Error)
		if ok {
			if apiError.Code == 404 {
				g.log.Infof("delete iam service account: service account %v does not exist", teamID)
				return nil
			}
		}
		return err
	}

	return nil
}
