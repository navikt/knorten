package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace                = "knada-system"
	saName                   = "knorten"
	ttlSecondsAfterFinished  = 60
	backoffLimit             = 1
	helmRepoConfigMap        = "helm-repos"
	helmRepoConfigMapSubPath = "repositories.yaml"
	helmRepoConfigMountPath  = "/root/.config/helm/repositories.yaml"
	cpuRequests              = "250m"
	memoryRequests           = "512Mi"
	ephemeralStorageRequests = "64Mi"
)

func (c *Client) CreateHelmInstallOrUpgradeJob(ctx context.Context, teamID, releaseName string, values map[string]any) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		chartType := helm.ReleaseNameToChartType(releaseName)
		out, err := yaml.Marshal(values)
		if err != nil {
			c.log.WithError(err).Errorf("error while marshaling chart for %v", chartType)
			return err
		}

		err = os.WriteFile(fmt.Sprintf("%v.yaml", chartType), out, 0o644)
		if err != nil {
			c.log.WithError(err).Errorf("error while writing to file %v.yaml", chartType)
			return err
		}
		return nil
	}

	encValues, err := c.encryptValues(values)
	if err != nil {
		return err
	}

	chartType := helm.ReleaseNameToChartType(releaseName)
	chartVersion, err := c.versionForChart(chartType)
	if err != nil {
		return err
	}

	job := c.createJobSpec(teamID, releaseName, string(helm.ActionInstallOrUpgrade))

	container := job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []v1.EnvFromSource{
		{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: "knelm",
				},
			},
		},
	}

	container.Env = []v1.EnvVar{
		{
			Name:  "HELM_VALUES",
			Value: encValues,
		},
		{
			Name:  "CHART_VERSION",
			Value: chartVersion,
		},
	}

	job.Spec.Template.Spec.Containers[0] = container

	_, err = c.clientSet.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if err := c.repo.TeamSetPendingUpgrade(ctx, teamID, helm.ReleaseNameToChartType(releaseName), true); err != nil {
		return err
	}

	return nil
}

func (c *Client) CreateHelmUninstallJob(ctx context.Context, teamID, releaseName string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	_, err := c.clientSet.BatchV1().Jobs(namespace).Create(ctx, c.createJobSpec(teamID, releaseName, string(helm.ActionUninstall)), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) createJobSpec(teamID, releaseName, action string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%v-%v-", teamID, releaseName),
			Namespace:    namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "knelm",
							Image: c.knelmImage,
							Command: []string{
								"/app/knelm",
								fmt.Sprintf("--action=%v", action),
								fmt.Sprintf("--releasename=%v", releaseName),
								fmt.Sprintf("--team=%v", teamID),
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "helm-repos-config",
									MountPath: helmRepoConfigMountPath,
									SubPath:   helmRepoConfigMapSubPath,
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									"cpu":               resource.MustParse(cpuRequests),
									"memory":            resource.MustParse(memoryRequests),
									"ephemeral-storage": resource.MustParse(ephemeralStorageRequests),
								},
							},
						},
					},
					RestartPolicy:      v1.RestartPolicyNever,
					ServiceAccountName: saName,
					Volumes: []v1.Volume{
						{
							Name: "helm-repos-config",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: helmRepoConfigMap,
									},
								},
							},
						},
					},
				},
			},
			TTLSecondsAfterFinished: intToInt32Ptr(ttlSecondsAfterFinished),
			BackoffLimit:            intToInt32Ptr(backoffLimit),
		},
	}

	return job
}

func (c *Client) encryptValues(values map[string]any) (string, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	valuesEncoded := base64.StdEncoding.EncodeToString(data)
	return c.cryptClient.EncryptValue(valuesEncoded)
}

func (c *Client) versionForChart(chartType string) (string, error) {
	switch chartType {
	case string(gensql.ChartTypeAirflow):
		return c.airflowChartVersion, nil
	case string(gensql.ChartTypeJupyterhub):
		return c.jupyterChartVersion, nil
	default:
		return "", fmt.Errorf("chart type %v does not exist", chartType)
	}
}

func intToInt32Ptr(val int) *int32 {
	valInt32 := int32(val)
	return &valInt32
}
