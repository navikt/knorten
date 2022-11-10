package chart

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/reflect"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
)

type NamespaceForm struct {
	// User config
	Namespace string   `form:"team" binding:"required" helm:"namespace"`
	Users     []string `form:"users[]" binding:"required"`

	// Generated config
	IAMServiceAccount string `helm:"iam.serviceAccount"`
	GSMSecret         string `helm:"gsm.secretName"`
}

func CreateNamespace(c *gin.Context, repo *database.Repo, helmClient *helm.Client, chartType gensql.ChartType, dryRun bool) error {
	googleClient := google.New(dryRun)

	var form NamespaceForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	_, err = repo.TeamGet(c, form.Namespace)
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("there already exists a team %v", form.Namespace)
	}

	if err := createGCPResources(c, &form, googleClient); err != nil {
		return err
	}

	chartValues, err := reflect.CreateChartValues(form)
	if err != nil {
		return err
	}

	if err := repo.TeamCreate(c, chartValues, form.Namespace, form.Users); err != nil {
		return err
	}

	application := helmApps.NewNamespace(form.Namespace, repo)
	_, err = application.Chart(c)
	if err != nil {
		return err
	}

	go helmClient.InstallOrUpgrade(c, string(chartType), form.Namespace, application)

	return nil
}

func UpdateNamespace(c *gin.Context, helmClient *helm.Client, repo *database.Repo) error {
	var form NamespaceForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	// oppdatere alle services dersom users er endret
	if err := repo.TeamUpdate(c, form.Namespace, form.Users); err != nil {
		return err
	}

	jupyterForm := JupyterForm{}
	if err := repo.TeamConfigurableValuesGet(c, gensql.ChartTypeNamespace, form.Namespace, &jupyterForm.JupyterValues); err != nil {
		return err
	}
	jupyterForm.AdminUsers = form.Users
	jupyterForm.AllowedUsers = form.Users

	if err := installOrUpdateJupyterhub(c, repo, helmClient, jupyterForm); err != nil {
		return err
	}

	return nil
}

func createGCPResources(c context.Context, form *NamespaceForm, googleClient *google.Google) error {
	if googleClient.DryRun {
		return nil
	}

	project := "projects/knada-gcp"

	iamSA, err := googleClient.CreateIAMServiceAccount(c, project, form.Namespace)
	if err != nil {
		return err
	}
	form.IAMServiceAccount = iamSA.Email

	gsmSecret, err := googleClient.CreateGSMSecret(c, project, form.Namespace)
	if err != nil {
		return err
	}
	form.GSMSecret = gsmSecret.Name

	if err := googleClient.CreateSASecretAccessorBinding(c, iamSA.Email, project+"/secrets/"+form.Namespace); err != nil {
		return err
	}

	if err := googleClient.CreateUserSecretOwnerBindings(c, form.Users, project+"/secrets/"+form.Namespace); err != nil {
		return err
	}

	if err := googleClient.CreateSAWorkloadIdentityBinding(c, iamSA.Email, form.Namespace); err != nil {
		return err
	}

	return nil
}
