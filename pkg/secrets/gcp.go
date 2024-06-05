package secrets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/navikt/knorten/pkg/gcp"
	"google.golang.org/grpc/codes"
)

func (e *ExternalSecretClient) GetTeamSecretGroups(ctx context.Context, gcpProject *string, teamID string) (map[string]*SecretGroup, error) {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	filter := fmt.Sprintf("labels.%v=true AND labels.%v=%v", externalSecretLabelKey, teamIDLabelKey, teamID)
	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, filter)
	if err != nil {
		return nil, err
	}

	teamSecretGroups := map[string]*SecretGroup{}
	for _, secret := range secrets {
		if group, ok := secret.Labels[secretGroupKey]; ok {
			secretVersion, err := gcp.GetLatestSecretVersion(ctx, secret.Name)
			if err != nil {
				return nil, err
			}

			if _, ok := teamSecretGroups[group]; !ok {
				gcpProject, err := projectFromSecretName(secretVersion.Name)
				if err != nil {
					return nil, err
				}
				teamSecretGroups[group] = &SecretGroup{
					GCPProject: gcpProject,
					Secrets:    []TeamSecret{},
				}
			}

			group := teamSecretGroups[group]
			group.Secrets = append(group.Secrets, TeamSecret{
				Key:   secret.Name,
				Value: string(secretVersion.Payload.Data),
				Name:  secretNameFromResourceName(secret.Name),
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

func (e *ExternalSecretClient) CreateOrUpdateTeamSecretGroup(gcpProject *string, teamID, group string, groupSecrets []TeamSecret) {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	ctxWithTimeout, cancel := context.WithTimeout(e.ctx, time.Second*20)
	defer cancel()

	for _, secret := range groupSecrets {
		s, err := e.getOrCreateSecret(ctxWithTimeout, projectID, teamID, group, secret.Name)
		if err != nil {
			e.log.Errorf("problem getting or creating secret for group %v for team %v: %v", group, teamID, err)
			return
		}
		if err := gcp.AddSecretVersion(ctxWithTimeout, s.Name, secret.Value); err != nil {
			e.log.Errorf("problem updating secret version for secret %v for team %v: %v", s.Name, teamID, err)
			return
		}
	}

	err := e.repo.RegisterApplyExternalSecret(ctxWithTimeout, teamID, EventData{
		TeamID:      teamID,
		SecretGroup: group,
	})
	if err != nil {
		e.log.Errorf("problem registering apply external secret event for team %v: %v", teamID, err)
		return
	}
}

func (e *ExternalSecretClient) deleteTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, secretGroup string) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, allSecretsInGroupFilter(teamID, secretGroup))
	if err != nil {
		return err
	}

	for _, secret := range secrets {
		if err := gcp.DeleteSecret(ctx, secret.Name); err != nil {
			return err
		}
	}

	return nil
}

func (e *ExternalSecretClient) getOrCreateSecret(ctx context.Context, projectID, teamID, group, secretName string) (*secretmanagerpb.Secret, error) {
	secret, err := gcp.GetSecret(ctx, projectID, createSecretID(teamID, group, secretName))
	if err != nil {
		apiError, ok := apierror.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.NotFound {
				return gcp.CreateSecret(ctx, projectID, e.defaultGCPLocation, createSecretID(teamID, group, secretName), createSecretLabels(teamID, group))
			}
		}
		return nil, err
	}

	return secret, nil
}

func createSecretID(teamID, group, secretName string) string {
	return fmt.Sprintf("%v-%v-%v-%v", knadaSecretPrefix, strings.ToLower(teamID), strings.ToLower(group), strings.ToLower(secretName))
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

func projectFromSecretName(secretName string) (string, error) {
	secretNameParts := strings.Split(secretName, "/")
	if len(secretNameParts) != 6 {
		return "", fmt.Errorf("unable to extract gcp project from secret name %v", secretName)
	}

	return secretNameParts[1], nil
}

func secretNameFromResourceName(resourceName string) string {
	parts := strings.Split(resourceName, "/")
	partsSecretName := strings.Split(parts[len(parts)-1], "-")
	return partsSecretName[len(partsSecretName)-1]
}
