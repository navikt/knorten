package helm_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/helm/mock"
)

func TestEnricher(t *testing.T) {
	t.Parallel()

	decrypted := "decrypted"

	testCases := []struct {
		name      string
		enricher  helm.Enricher
		values    map[string]any
		filter    cmp.Option
		expect    any
		expectErr bool
	}{
		// Global
		{
			name: "global: with no errors or values",
			enricher: helm.NewGlobalEnricher(
				"test",
				mock.NewEnricherStore(nil, nil, nil, nil),
			),
			values: map[string]any{},
			expect: map[string]any{},
		},
		{
			name: "global: with error",
			enricher: helm.NewGlobalEnricher(
				"test",
				mock.NewEnricherStore(nil, nil, nil, fmt.Errorf("oops")),
			),
			expectErr: true,
		},
		{
			name: "global: with stored values",
			enricher: helm.NewGlobalEnricher(
				"test",
				mock.NewEnricherStore(nil, &gensql.ChartGlobalValue{Key: "global", Value: "value"}, nil, nil),
			),
			values: map[string]any{},
			expect: map[string]any{"global": "value"},
		},
		{
			name: "global: with stored and existing values",
			enricher: helm.NewGlobalEnricher(
				"test",
				mock.NewEnricherStore(nil, &gensql.ChartGlobalValue{Key: "global", Value: "value"}, nil, nil),
			),
			values: map[string]any{"global": "old"},
			expect: map[string]any{"global": "value"},
		},
		{
			name: "global: with decrypted stored values",
			enricher: helm.NewGlobalEnricher(
				"test",
				mock.NewEnricherStore(&decrypted, &gensql.ChartGlobalValue{Key: "global", Value: "enc", Encrypted: true}, nil, nil),
			),
			values: map[string]any{},
			expect: map[string]any{"global": "decrypted"},
		},
		// Team
		{
			name: "team: with no errors or values",
			enricher: helm.NewTeamEnricher(
				"test",
				"team",
				mock.NewEnricherStore(nil, nil, nil, nil),
			),
			values: map[string]any{},
			expect: map[string]any{},
		},
		{
			name: "team: with error",
			enricher: helm.NewTeamEnricher(
				"test",
				"team",
				mock.NewEnricherStore(nil, nil, nil, fmt.Errorf("oops")),
			),
			expectErr: true,
		},
		{
			name: "team: with stored values",
			enricher: helm.NewTeamEnricher(
				"test",
				"team",
				mock.NewEnricherStore(nil, nil, &gensql.ChartTeamValue{Key: "team", Value: "value"}, nil),
			),
			values: map[string]any{},
			expect: map[string]any{"team": "value"},
		},
		{
			name: "team: with stored and existing values",
			enricher: helm.NewTeamEnricher(
				"test",
				"team",
				mock.NewEnricherStore(nil, nil, &gensql.ChartTeamValue{Key: "team", Value: "value"}, nil),
			),
			values: map[string]any{"team": "old"},
			expect: map[string]any{"team": "value"},
		},
		{
			name: "team: with fernetKey that should be skipped",
			enricher: helm.NewTeamEnricher(
				"test",
				"team",
				mock.NewEnricherStore(nil, nil, &gensql.ChartTeamValue{Key: "fernetKey", Value: "value"}, nil),
			),
			values: map[string]any{},
			expect: map[string]any{},
		},
		{
			name: "jupyterhub: with no errors or values",
			enricher: helm.NewJupyterhubEnricher(
				"team",
				mock.NewEnricherStore(nil, nil, nil, sql.ErrNoRows),
			),
			values: map[string]any{},
			expect: map[string]any{},
		},
		{
			name: "jupyterhub: with error",
			enricher: helm.NewJupyterhubEnricher(
				"team",
				mock.NewEnricherStore(nil, nil, nil, fmt.Errorf("oops")),
			),
			expectErr: true,
		},
		{
			name: "jupyterhub: with stored values",
			enricher: helm.NewJupyterhubEnricher(
				"team",
				mock.NewEnricherStore(nil, nil, nil, nil).
					SetTeamValue(helm.ProfileListKey, gensql.ChartTeamValue{
						Key:   helm.ProfileListKey,
						Value: "[]",
					}).
					SetGlobalValue(helm.ProfileListKey, gensql.ChartGlobalValue{
						Key:   helm.ProfileListKey,
						Value: "[{\"profile\": \"value\"}]",
					}),
			),
			values: map[string]any{},
			expect: map[string]any{
				"singleuser": map[string]any{
					"profileList": []map[string]any{
						{"profile": "value"},
					},
				},
			},
		},
		{
			name: "airflow: with no errors or values",
			enricher: helm.NewAirflowEnricher(
				"team",
				mock.NewEnricherStore(nil, nil, nil, nil).
					SetGlobalValue(helm.KnauditImageKey, gensql.ChartGlobalValue{
						Key:   helm.KnauditImageKey,
						Value: "knaudit:latest",
					}).
					SetGlobalValue(helm.EnvKey, gensql.ChartGlobalValue{
						Key:   helm.EnvKey,
						Value: "[]",
					}).
					SetTeamValue(helm.EnvKey, gensql.ChartTeamValue{
						Key:   helm.EnvKey,
						Value: "[]",
					}),
			),
			values: map[string]any{},
			expect: map[string]any{},
			filter: cmp.FilterPath(func(p cmp.Path) bool {
				return strings.Contains(p.GoString(), "workers")
			}, cmp.Ignore()),
		},
		{
			name: "airflow: with error",
			enricher: helm.NewAirflowEnricher(
				"team",
				mock.NewEnricherStore(nil, nil, nil, fmt.Errorf("oops")),
			),
			expectErr: true,
		},
		{
			name: "airflow: with values",
			enricher: helm.NewAirflowEnricher(
				"team",
				mock.NewEnricherStore(nil, nil, nil, nil).
					SetGlobalValue(helm.KnauditImageKey, gensql.ChartGlobalValue{
						Key:   helm.KnauditImageKey,
						Value: "knaudit:latest",
					}).
					SetGlobalValue(helm.EnvKey, gensql.ChartGlobalValue{
						Key:   helm.EnvKey,
						Value: "[{\"globalKey\": \"value\"}]",
					}).
					SetTeamValue(helm.EnvKey, gensql.ChartTeamValue{
						Key:   helm.EnvKey,
						Value: "[{\"teamKey\": \"teamValue\"}]",
					}),
			),
			values: map[string]any{},
			expect: map[string]any{
				"env": []map[string]string{
					{"globalKey": "value"},
					{"teamKey": "teamValue"},
				},
			},
			filter: cmp.FilterPath(func(p cmp.Path) bool {
				return strings.Contains(p.GoString(), "workers")
			}, cmp.Ignore()),
		},
		{
			name: "chain: with no errors or values",
			enricher: helm.NewChainEnricher(
				helm.NewGlobalEnricher(
					"test",
					mock.NewEnricherStore(nil, nil, nil, nil),
				),
				helm.NewTeamEnricher(
					"test",
					"team",
					mock.NewEnricherStore(nil, nil, nil, nil),
				),
				helm.NewJupyterhubEnricher(
					"team",
					mock.NewEnricherStore(nil, nil, nil, sql.ErrNoRows),
				),
			),
			values: map[string]any{},
			expect: map[string]any{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.enricher.Enrich(context.Background(), tc.values)
			if tc.expectErr {
				if err == nil {
					t.Errorf("enrich: expected error %v, got %v", tc.expectErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("enrich: got unexpected error %v", err)
				}

				if diff := cmp.Diff(tc.expect, got, tc.filter); diff != "" {
					t.Errorf("enrich: (-expect +got)\n%s", diff)
				}
			}
		})
	}
}
