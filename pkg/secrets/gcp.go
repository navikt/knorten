package secrets

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/navikt/knorten/pkg/gcp"
	"google.golang.org/grpc/codes"
)

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

func (e *ExternalSecretClient) GetTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, secretGroup string) ([]TeamSecret, error) {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, allSecretsInGroupFilter(teamID, secretGroup))
	if err != nil {
		return nil, err
	}

	teamSecrets := []TeamSecret{}
	for _, secret := range secrets {
		secretVersion, err := gcp.GetLatestSecretVersion(ctx, secret.Name)
		if err != nil {
			return nil, err
		}

		teamSecrets = append(teamSecrets, TeamSecret{
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

func (e *ExternalSecretClient) CreateOrUpdateTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, group string, secrets []TeamSecret) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	for _, secret := range secrets {
		s, err := e.getOrCreateSecret(ctx, projectID, teamID, group, secret.Key)
		if err != nil {
			return err
		}
		if err := gcp.AddSecretVersion(ctx, projectID, e.defaultGCPLocation, s.Name, secret.Value); err != nil {
			return err
		}
	}

	return nil
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

func (e *ExternalSecretClient) getOrCreateSecret(ctx context.Context, projectID, teamID, group, secretName string) (*secretmanagerpb.Secret, error) {
	secret, err := gcp.GetSecret(ctx, secretName)
	if err != nil {
		apiError, ok := apierror.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.NotFound {
				return gcp.CreateSecret(ctx, projectID, e.defaultGCPLocation, createSecretID(teamID, secretName), createSecretLabels(teamID, group))
			}
		}
		return nil, err
	}

	return secret, nil
}

func createSecretID(teamID, secretName string) string {
	return fmt.Sprintf("%v-%v-%v", knadaSecretPrefix, strings.ToLower(teamID), strings.ToLower(secretName))
}

func createSecretLabels(teamID, secretGroup string) map[string]string {
	return map[string]string{
		teamIDLabelKey:         teamID,
		secretGroupKey:         secretGroup,
		externalSecretLabelKey: "true",
	}
}

func allSecretsInGroupFilter(teamID, secretGroup string) string {
	return fmt.Sprintf("labels.%v=true AND labels.%v=%v AND labels.%v=%v", externalSecretLabelKey, teamIDLabelKey, teamID, secretGroupKey, secretGroup)
}
