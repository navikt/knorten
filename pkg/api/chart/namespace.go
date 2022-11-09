package chart

import (
	"context"
	"fmt"
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

type NamespaceJupyterhubConfig struct {
	Enabled bool `helm:"jupyterhub.enabled"`
}

func NamespaceCreate(c context.Context, repo *database.Repo, helmClient *helm.Client, chartType gensql.ChartType) error {
	googleClient := google.New()

	// todo form in ui
	form := NamespaceForm{
		Namespace: "nada-test-til-sletting",
		Users:     []string{},
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

	formValues := reflect.ValueOf(form)
	formFields := reflect.VisibleFields(reflect.TypeOf(form))
	chartValues, err := createChartValues(formValues, formFields)
	if err != nil {
		return err
	}

	if err := repo.NamespaceCreate(c, gensql.ChartTypeNamespace, chartValues, form.Namespace); err != nil {
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

func NamespaceAddJupyterhub(c context.Context, repo *database.Repo, helmClient *helm.Client, team string) error {
	jupyterConfig := NamespaceJupyterhubConfig{
		Enabled: true,
	}

	existing, err := repo.TeamValuesGet(c, gensql.ChartTypeNamespace, form.Namespace)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("there already exists a namespace %v", form.Namespace)
	}

	formValues := reflect.ValueOf(jupyterConfig)
	formFields := reflect.VisibleFields(reflect.TypeOf(jupyterConfig))
	chartValues, err := createChartValues(formValues, formFields)
	if err != nil {
		return err
	}

	if err := repo.NamespaceUpdate(c, gensql.ChartTypeNamespace, chartValues, team); err != nil {
		return err
	}

	googleClient := google.New()
	googleClient.CreateSAWorkloadIdentityBinding(c, iamSA string, namespace string)

	namespace := helmNS.NewNamespace(team, repo)
	_, err = namespace.Chart(c)
	if err != nil {
		return err
	}

	go helmClient.InstallOrUpgrade(c, string(gensql.ChartTypeNamespace), team, namespace)

	return nil
}

func currentConfig()  {

}

func createGCPResources(c context.Context, form *NamespaceForm, googleClient *google.Google) error {
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

func CreateJupyterhubGCPResources(ctx context.Context) error {
}
