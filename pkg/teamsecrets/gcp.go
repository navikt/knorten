package teamsecrets

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/navikt/knorten/pkg/gcp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
)

const allowedGSMNameRegex = `[^\d^\w^\-^_]`

type SafeSecretGroups struct {
	mutex            sync.Mutex
	teamSecretGroups map[string]*SecretGroup
}

func (ssg *SafeSecretGroups) appendForGroup(group string, secretVersion *secretmanagerpb.AccessSecretVersionResponse) {
	ssg.mutex.Lock()
	defer ssg.mutex.Unlock()

	if _, ok := ssg.teamSecretGroups[group]; !ok {
		gcpProject, err := projectFromSecretName(secretVersion.Name)
		if err != nil {
			return
		}
		ssg.teamSecretGroups[group] = &SecretGroup{
			GCPProject: gcpProject,
			Secrets:    []TeamSecret{},
		}
	}

	secretGroup := ssg.teamSecretGroups[group]
	secretGroup.Secrets = append(secretGroup.Secrets, TeamSecret{
		Key:   secretVersion.Name,
		Value: string(secretVersion.Payload.Data),
		Name:  secretNameFromResourceName(secretVersion.Name),
	})
}

func (e *TeamSecretClient) GetTeamSecretGroups(ctx context.Context, gcpProject *string, teamID string) (map[string]*SecretGroup, error) {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	filter := fmt.Sprintf("labels.%v=true AND labels.%v=%v", externalSecretLabelKey, teamIDLabelKey, teamID)
	secrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, filter)
	if err != nil {
		return nil, err
	}

	safeSecretGroup := SafeSecretGroups{
		teamSecretGroups: map[string]*SecretGroup{},
	}
	g, ctx := errgroup.WithContext(ctx)
	for _, secret := range secrets {
		if group, ok := secret.Labels[secretGroupKey]; ok {
			s := secret
			g.Go(func() error {
				secretVersion, err := gcp.GetLatestSecretVersion(ctx, s.Name)
				if err != nil {
					return fmt.Errorf("getting latest secret version for secret %v: %v", s.Name, err)
				}
				safeSecretGroup.appendForGroup(group, secretVersion)
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return safeSecretGroup.teamSecretGroups, nil
}

func (e *TeamSecretClient) GetTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, secretGroup string) ([]TeamSecret, error) {
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

func (e *TeamSecretClient) CreateOrUpdateTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, group string, groupSecrets []TeamSecret) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	existingSecrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, allSecretsInGroupFilter(teamID, group))
	if err != nil {
		return err
	}

	g, createCtx := errgroup.WithContext(ctx)
	for _, groupSecret := range groupSecrets {
		gs := groupSecret
		g.Go(func() error {
			s, err := e.getOrCreateSecret(createCtx, projectID, teamID, group, gs.Name)
			if err != nil {
				return fmt.Errorf("problem getting or creating secret for group %v for team %v: %w", group, teamID, err)
			}
			if err := gcp.AddSecretVersion(createCtx, s.Name, gs.Value); err != nil {
				return fmt.Errorf("problem updating secret version for secret %v for team %v: %w", s.Name, teamID, err)
			}
			existingSecrets = removeFromExisting(existingSecrets, s)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	g, deleteCtx := errgroup.WithContext(ctx)
	for _, existingSecret := range existingSecrets {
		es := existingSecret
		fmt.Println(es)
		g.Go(func() error {
			if err := gcp.DeleteSecret(deleteCtx, es.Name); err != nil {
				return fmt.Errorf("problem deleting secret %v for team %v: %w", es.Name, teamID, err)
			}
			return nil
		})
	}

	return g.Wait()
}

func (e *TeamSecretClient) DeleteTeamSecretGroup(ctx context.Context, gcpProject *string, teamID, group string) error {
	projectID := e.defaultGCPProject
	if gcpProject != nil {
		projectID = *gcpProject
	}

	existingSecrets, err := gcp.ListSecrets(ctx, teamID, projectID, e.defaultGCPLocation, allSecretsInGroupFilter(teamID, group))
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, existingSecret := range existingSecrets {
		es := existingSecret
		g.Go(func() error {
			if err := gcp.DeleteSecret(ctx, es.Name); err != nil {
				return fmt.Errorf("problem deleting secret %v for team %v: %w", es.Name, teamID, err)
			}
			return nil
		})
	}

	return g.Wait()
}

func (e *TeamSecretClient) getOrCreateSecret(ctx context.Context, projectID, teamID, group, secretName string) (*secretmanagerpb.Secret, error) {
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
	return fmt.Sprintf("%v-%v-%v-%v", knadaSecretPrefix, strings.ToLower(teamID), group, secretName)
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
	partsSecretName := strings.Split(parts[len(parts)-3], "-")
	return partsSecretName[len(partsSecretName)-1]
}

func removeFromExisting(existingSecrets []*secretmanagerpb.Secret, secret *secretmanagerpb.Secret) []*secretmanagerpb.Secret {
	for idx, es := range existingSecrets {
		if es.Name == secret.Name {
			return append(existingSecrets[:idx], existingSecrets[idx+1:]...)
		}
	}

	return existingSecrets
}

func FormatGroupName(group string) string {
	re := regexp.MustCompile(allowedGSMNameRegex)
	lowerCaseWithUnderscoreReplaced := strings.ReplaceAll(strings.ToLower(group), "_", "-")
	return re.ReplaceAllString(lowerCaseWithUnderscoreReplaced, "")
}

func FormatSecretName(secretName string) string {
	re := regexp.MustCompile(allowedGSMNameRegex)
	upperCaseWithDashesReplaced := strings.ReplaceAll(strings.ToUpper(secretName), "-", "_")
	return re.ReplaceAllString(upperCaseWithDashesReplaced, "")
}
