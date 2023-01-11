package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/knorten/pkg/database/gensql"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JupyterProfileList struct {
	KubespawnerOverride struct {
		Image string `json:"image"`
	} `json:"kubespawner_override"`
}

const (
	kipCPU          = "10m"
	kipMemory       = "20Mi"
	imagePullSecret = "ghcr-credentials"
)

func (c *Client) CreateOrUpdateKIPDaemonset(ctx context.Context) error {
	initContainers, err := c.createImagePullInitContainers(ctx)
	if err != nil {
		return err
	}
	if len(initContainers) <= 0 {
		c.log.Info("no images to pull, not deploying kip")
		return nil
	}

	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kip",
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"daemonset": "kip"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "kip-",
					Namespace:    namespace,
					Labels:       map[string]string{"daemonset": "kip"},
				},
				Spec: v1.PodSpec{
					InitContainers: initContainers,
					Containers: []v1.Container{
						{
							Name:  "pause",
							Image: "k8s.gcr.io/pause:3.8",
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									"cpu":    resource.MustParse(kipCPU),
									"memory": resource.MustParse(kipMemory),
								},
								Limits: v1.ResourceList{
									"cpu":    resource.MustParse(kipCPU),
									"memory": resource.MustParse(kipMemory),
								},
							},
						},
					},
					ImagePullSecrets: []v1.LocalObjectReference{
						{
							Name: imagePullSecret,
						},
					},
				},
			},
		},
	}

	dss, err := c.clientSet.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, existing := range dss.Items {
		if existing.Name == daemonset.Name {
			return c.updateKIPDaemonset(ctx, daemonset)
		}
	}

	_, err = c.clientSet.AppsV1().DaemonSets(namespace).Create(ctx, daemonset, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) createImagePullInitContainers(ctx context.Context) ([]v1.Container, error) {
	profiles, err := c.getProfiles(ctx)
	if err != nil {
		return nil, err
	}

	initContainers := []v1.Container{}
	for idx, p := range profiles {
		initContainers = append(initContainers, v1.Container{
			Name:    fmt.Sprintf("puller-%v", idx),
			Image:   p.KubespawnerOverride.Image,
			Command: []string{"/bin/sh", "-c", "echo pull complete"},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"cpu":    resource.MustParse(kipCPU),
					"memory": resource.MustParse(kipMemory),
				},
				Limits: v1.ResourceList{
					"cpu":    resource.MustParse(kipCPU),
					"memory": resource.MustParse(kipMemory),
				},
			},
		})
	}

	return initContainers, nil
}

func (c *Client) getProfiles(ctx context.Context) ([]JupyterProfileList, error) {
	globVals, err := c.repo.GlobalValuesGet(ctx, gensql.ChartTypeJupyterhub)
	if err != nil {
		return nil, err
	}

	var profiles []JupyterProfileList
	for _, gv := range globVals {
		if gv.Key == "singleuser.profileList" {
			profiles, err = extractProfiles(gv.Value)
			if err != nil {
				return nil, err
			}
		}
	}

	return profiles, nil
}

func (c *Client) updateKIPDaemonset(ctx context.Context, daemonset *appsv1.DaemonSet) error {
	_, err := c.clientSet.AppsV1().DaemonSets(namespace).Update(ctx, daemonset, metav1.UpdateOptions{})
	return err
}

func extractProfiles(profileListString string) ([]JupyterProfileList, error) {
	profiles := []JupyterProfileList{}

	if err := json.Unmarshal([]byte(profileListString), &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}
