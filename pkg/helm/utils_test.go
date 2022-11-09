package helm

import (
	"reflect"
	"testing"
)

func Test_SetChartValue(t *testing.T) {
	type args struct {
		keys  []string
		value string
		chart map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "Simple test",
			args: args{
				keys:  []string{"singleuser", "image", "name"},
				value: "navikt/jupyter",
				chart: map[string]any{"singleuser": map[string]any{"image": map[string]any{"name": "jupyter"}}},
			},
			want: map[string]any{"singleuser": map[string]any{"image": map[string]any{"name": "navikt/jupyter"}}},
		},
		{
			name: "No missing values",
			args: args{
				keys:  []string{"singleuser", "image", "name"},
				value: "navikt/jupyter",
				chart: map[string]any{"singleuser": map[string]any{"image": map[string]any{"name": "jupyter", "tag": "v1"}}},
			},
			want: map[string]any{"singleuser": map[string]any{"image": map[string]any{"name": "navikt/jupyter", "tag": "v1"}}},
		},
		{
			name: "Missing nested path in chart values",
			args: args{
				keys:  []string{"hub", "config", "AzureAdOAuthenticator", "tenant_id"},
				value: "id",
				chart: map[string]any{"hub": map[string]any{"config": map[string]any{"Jupyterhub": map[string]any{"authenticator_class": "dummy"}}}},
			},
			want: map[string]any{"hub": map[string]any{"config": map[string]any{"Jupyterhub": map[string]any{"authenticator_class": "dummy"}, "AzureAdOAuthenticator": map[string]any{"tenant_id": "id"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetChartValue(tt.args.keys, tt.args.value, tt.args.chart)
			if !reflect.DeepEqual(tt.args.chart, tt.want) {
				t.Errorf("setChartValue() = %v, want %v", tt.args.chart, tt.want)
			}
		})
	}
}
