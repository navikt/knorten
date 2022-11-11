package team

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/k8s"
)

type TeamForm struct {
	// User config
	Team  string   `form:"team" binding:"required"`
	Users []string `form:"users[]" binding:"required"`
}

func Create(c *gin.Context, repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client) error {
	var form TeamForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	_, err = repo.TeamGet(c, form.Team)
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("there already exists a team %v", form.Team)
	}

	if err := createGCPResources(c, &form, googleClient); err != nil {
		return err
	}

	if err := repo.TeamCreate(c, form.Team, form.Users); err != nil {
		return err
	}

	if err := k8sClient.CreateTeamNamespace(c, form.Team); err != nil {
		return err
	}

	return nil
}

func createGCPResources(c context.Context, form *TeamForm, googleClient *google.Google) error {
	if googleClient.DryRun {
		return nil
	}

	project := "projects/knada-gcp"

	iamSA, err := googleClient.CreateIAMServiceAccount(c, project, form.Team)
	if err != nil {
		return err
	}

	gsmSecret, err := googleClient.CreateGSMSecret(c, project, form.Team)
	if err != nil {
		return err
	}

	if err := googleClient.CreateSASecretAccessorBinding(c, iamSA.Email, gsmSecret.Name); err != nil {
		return err
	}

	if err := googleClient.CreateUserSecretOwnerBindings(c, form.Users, gsmSecret.Name); err != nil {
		return err
	}

	if err := googleClient.CreateSAWorkloadIdentityBinding(c, iamSA.Email, form.Team); err != nil {
		return err
	}

	return nil
}
