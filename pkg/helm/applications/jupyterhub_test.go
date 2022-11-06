package helm

import (
	"reflect"
	"testing"
)

func Test_setChartValue(t *testing.T) {
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
			setChartValue(tt.args.keys, tt.args.value, tt.args.chart)
			if !reflect.DeepEqual(tt.args.chart, tt.want) {
				t.Errorf("setChartValue() = %v, want %v", tt.args.chart, tt.want)
			}
		})
	}
}

func Test_parseValue(t *testing.T) {
	type args struct {
		value any
	}
	tests := []struct {
		name string
		args args
		want any
	}{
		{
			name: "String test",
			args: args{
				value: "navikt/jupyter",
			},
			want: "navikt/jupyter",
		},
		{
			name: "Map test",
			args: args{
				value: "{key:value,key2:value2}",
			},
			want: map[string]any{"key": "value", "key2": "value2"},
		},
		{
			name: "List test",
			args: args{
				value: "[item1,item2,item3]",
			},
			want: []any{"item1", "item2", "item3"},
		},
		{
			name: "Combined nested test",
			args: args{
				value: "[{key1:value1,key2:[item1,item2]},{key:value},item3]",
			},
			want: []any{map[string]any{"key1": "value1", "key2": []any{"item1", "item2"}}, map[string]any{"key": "value"}, "item3"},
		},
		{
			name: "Combined nested test",
			args: args{
				value: "[{key1:{key2:value},key2:[item1,item2]},{key:value},item3]",
			},
			want: []any{map[string]any{"key1": map[string]any{"key2": "value"}, "key2": []any{"item1", "item2"}}, map[string]any{"key": "value"}, "item3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := parseValue(tt.args.value)
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("parseValue() = %v, want %v", actual, tt.want)
			}
		})
	}
}
