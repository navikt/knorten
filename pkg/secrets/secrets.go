package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/navikt/knorten/pkg/gcp"
)

const (
	knadaSecretPrefix      = "knada"
	externalSecretLabelKey = "external-secret"
	teamIDLabelKey         = "team-id"
	secretGroupKey         = "secret-group"
)

type TeamSecret struct {
	Key   string
	Value string
}

type ExternalSecretClient struct {
	defaultGCPProject  string
	defaultGCPLocation string
}

func New(defaultGCPProject, defaultGCPLocation string) *ExternalSecretClient {
	return &ExternalSecretClient{
		defaultGCPProject:  defaultGCPProject,
		defaultGCPLocation: defaultGCPLocation,
	}
}

func (e *ExternalSecretClient) GetTeamSecretGroups(ctx context.Context, gcpProject *string, teamID string) (map[string][]TeamSecret, error) {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	filter := fmt.Sprintf("labels.%v=true AND labels.%v=%v", externalSecretLabelKey, teamIDLabelKey, teamID)
	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, filter)
	if err != nil {
		return nil, err
	}

	teamSecretGroups := map[string][]TeamSecret{}
	for _, secret := range secrets {
		if group, ok := secret.Labels[secretGroupKey]; ok {
			secretVersion, err := gcp.GetLatestSecretVersion(ctx, secret.Name)
			if err != nil {
				return nil, err
			}
			teamSecretGroups[group] = append(teamSecretGroups[group], TeamSecret{
				Key:   secret.Name,
				Value: string(secretVersion.Payload.Data),
			})
		}
	}

	return teamSecretGroups, nil
}

func (e *ExternalSecretClient) GetTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, secretGroup string) ([]*TeamSecret, error) {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	filter := fmt.Sprintf("labels.%v=true AND labels.%v=%v AND labels.%v=%v", externalSecretLabelKey, teamIDLabelKey, teamID, secretGroupKey, secretGroup)
	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, filter)
	if err != nil {
		return nil, err
	}

	teamSecrets := []*TeamSecret{}
	for _, secret := range secrets {
		secretVersion, err := gcp.GetLatestSecretVersion(ctx, secret.Name)
		if err != nil {
			return nil, err
		}

		teamSecrets = append(teamSecrets, &TeamSecret{
			Key:   secret.Name,
			Value: string(secretVersion.Payload.Data),
		})
	}

	return teamSecrets, nil
}

func (e *ExternalSecretClient) NewTeamSecret(ctx context.Context, gcpProject *string, teamID, group, secretName, secretValue string) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	secret, err := gcp.CreateSecret(ctx, projectID, e.defaultGCPLocation, createSecretID(teamID, secretName), map[string]string{
		teamIDLabelKey:         teamID,
		externalSecretLabelKey: "true",
	})
	if err != nil {
		return err
	}

	return gcp.AddSecretVersion(ctx, projectID, e.defaultGCPLocation, secret.Name, secretValue)
}

func (e *ExternalSecretClient) UpdateTeamSecret(ctx context.Context, gcpProject *string, teamID, group, secretName, secretValue string) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	secret, err := gcp.GetSecret(ctx, secretName)
	if err != nil {
		return err
	}

	return gcp.AddSecretVersion(ctx, projectID, e.defaultGCPLocation, secret.Name, secretValue)
}

func (e *ExternalSecretClient) DeleteTeamSecretGroups(ctx context.Context, gcpProject *string, teamID string) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	filter := fmt.Sprintf("labels.%v=true AND labels.%v=%v", externalSecretLabelKey, teamIDLabelKey, teamID)
	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, filter)
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if err := gcp.DeleteSecret(ctx, projectID, secret.Name); err != nil {
			return err
		}
	}

	return nil
}

func createSecretID(teamID, secretName string) string {
	return fmt.Sprintf("%v-%v-%v", knadaSecretPrefix, strings.ToLower(teamID), strings.ToLower(secretName))
}

func createDefaultEnvKey(secretName string) string {
	parts := strings.Split(secretName, "/")
	return strings.ToUpper(fmt.Sprintf("%v_%v", knadaSecretPrefix, parts[len(parts)-1]))
}
