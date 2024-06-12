package service

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/teamsecrets"
)

type GSMService interface {
	GetTeamSecretGroups(ctx context.Context, gcpProject *string, teamSlug string) (map[string]*teamsecrets.SecretGroup, error)
	CreateOrUpdateTeamSecretGroup(ctx context.Context, gcpProject *string, teamSlug, secretGroup string, groupSecrets []teamsecrets.TeamSecret) error
	DeleteTeamSecretGroup(ctx context.Context, gcpProject *string, teamSlug, secretGroup string) error
}

type GSMRepo interface {
	TeamBySlugGet(ctx context.Context, slug string) (gensql.TeamBySlugGetRow, error)
	RegisterApplyExternalSecret(ctx context.Context, teamID string, event any) error
	RegisterDeleteExternalSecret(ctx context.Context, teamID string, event any) error
}

type gsmService struct {
	gsmClient *teamsecrets.TeamSecretClient
	repo      GSMRepo
}

func NewGSMService(gsmClient *teamsecrets.TeamSecretClient, repo GSMRepo) *gsmService {
	return &gsmService{
		gsmClient: gsmClient,
		repo:      repo,
	}
}

func (gs *gsmService) GetTeamSecretGroups(ctx context.Context, gcpProject *string, teamSlug string) (map[string]*teamsecrets.SecretGroup, error) {
	team, err := gs.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("problem getting team from slug %v: %w", teamSlug, err)
	}

	secretGroups, err := gs.gsmClient.GetTeamSecretGroups(ctx, gcpProject, team.ID)
	if err != nil {
		return nil, fmt.Errorf("problem getting secret groups for team id %v: %v", team.ID, err)
	}

	return secretGroups, nil
}

func (gs *gsmService) CreateOrUpdateTeamSecretGroup(ctx context.Context, gcpProject *string, teamSlug, secretGroup string, groupSecrets []teamsecrets.TeamSecret) error {
	team, err := gs.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return fmt.Errorf("problem getting team from slug %v: %w", teamSlug, err)
	}

	if err := gs.gsmClient.CreateOrUpdateTeamSecretGroup(ctx, gcpProject, team.ID, secretGroup, groupSecrets); err != nil {
		return fmt.Errorf("problem creating or updating secret group %v for team %v: %v", secretGroup, team.ID, err)
	}

	err = gs.repo.RegisterApplyExternalSecret(ctx, team.ID, teamsecrets.EventData{
		TeamID:      team.ID,
		SecretGroup: secretGroup,
	})
	if err != nil {
		return fmt.Errorf("problem registering create or update external secret event for secret group %v, team %v: %v", secretGroup, team.ID, err)
	}

	return nil
}

func (gs *gsmService) DeleteTeamSecretGroup(ctx context.Context, gcpProject *string, teamSlug, secretGroup string) error {
	team, err := gs.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return fmt.Errorf("problem getting team from slug %v: %w", teamSlug, err)
	}

	if err := gs.gsmClient.DeleteTeamSecretGroup(ctx, gcpProject, team.ID, secretGroup); err != nil {
		return fmt.Errorf("problem deleting secret group %v for team %v: %v", secretGroup, team.ID, err)
	}

	err = gs.repo.RegisterDeleteExternalSecret(ctx, team.ID, teamsecrets.EventData{
		TeamID:      team.ID,
		SecretGroup: secretGroup,
	})
	if err != nil {
		return fmt.Errorf("problem registering delete external secret event for secret group %v, team %v: %v", secretGroup, team.ID, err)
	}

	return nil
}
