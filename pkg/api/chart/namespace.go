package chart

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"reflect"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	helmNS "github.com/nais/knorten/pkg/helm/namespace"
)

type NamespaceForm struct {
	// User config
	Namespace string   `form:"team" binding:"required" helm:"namespace"`
	Users     []string `form:"users[]" binding:"required" helm:"users"`

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

	existing, err := repo.TeamValuesGet(c, gensql.ChartTypeNamespace, form.Namespace)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("there already exists a namespace %v", form.Namespace)
	}

	if err := createGCPResources(c, &form, googleClient); err != nil {
		return err
	}

	values := reflect.ValueOf(form)
	fields := reflect.VisibleFields(reflect.TypeOf(form))
	chartValues, err := createChartValues(values, fields)
	if err != nil {
		return err
	}

	if err := repo.ApplicationCreate(c, gensql.ChartTypeNamespace, chartValues, form.Namespace, form.Users); err != nil {
		return err
	}

	namespace := helmNS.NewNamespace(form.Namespace, repo)
	_, err = namespace.Chart(c)
	if err != nil {
		return err
	}

	go helmClient.InstallOrUpgrade(c, string(chartType), form.Namespace, namespace)

	return nil
}

func UpdateNamespace(c *gin.Context, repo *database.Repo) error {
	var form NamespaceForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	values := reflect.ValueOf(form)
	fields := reflect.VisibleFields(reflect.TypeOf(form))
	chartValues, err := createChartValues(values, fields)
	if err != nil {
		return err
	}

	return repo.ApplicationCreate(c, gensql.ChartTypeNamespace, chartValues, form.Namespace, form.Users)
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

	return nil
}
