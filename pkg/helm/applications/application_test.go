package helm

import (
	"reflect"
	"testing"
)

func Test_parseTeamValue(t *testing.T) {
	type args struct {
		key    string
		value  any
		values map[string]any
	}
	tests := []struct {
		name string
		args args
		want any
	}{
		{
			name: "Simple test",
			args: args{
				key:    "webserver.name",
				value:  "flowtheair",
				values: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag"}},
			},
			want: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag", "name": "flowtheair"}},
		},
		{
			name: "Replace nested value test",
			args: args{
				key:    "webserver.extraContainers.[0].args",
				value:  "[\"navikt/repo\",\"main\",\"/dags\",\"60\"]",
				values: map[string]any{"webserver": map[string]any{"extraContainers": []any{map[string]any{"name": "hello"}}}},
			},
			want: map[string]any{"webserver": map[string]any{"extraContainers": []any{map[string]any{"name": "hello", "args": []any{"navikt/repo", "main", "/dags", "60"}}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTeamValue(tt.args.key, tt.args.value, tt.args.values)
			if !reflect.DeepEqual(tt.args.values, tt.want) {
				t.Errorf("parse() = %v, want %v", tt.args.values, tt.want)
			}
		})
	}
}

func Test_mergeMap(t *testing.T) {
	type args struct {
		base   map[string]any
		custom map[string]any
	}
	tests := []struct {
		name string
		args args
		want map[string]any
	}{
		{
			name: "Simple test",
			args: args{
				base:   map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag"}},
				custom: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "name": "flowtheair"}},
			},
			want: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "name": "flowtheair"}},
		},
		{
			name: "With slice",
			args: args{
				base:   map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag", "myslice": []any{"one", "two"}, "scheduler": "1234"}},
				custom: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "name": "flowtheair"}},
			},
			want: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "name": "flowtheair", "myslice": []any{"one", "two"}, "scheduler": "1234"}},
		},
		{
			name: "Nested test",
			args: args{
				base:   map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag"}, "scheduler": map[string]any{"image": "ghcr.io/org/repository", "values": []any{"1", "2"}}},
				custom: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "slice": []any{"12"}}, "scheduler": map[string]any{"values": []any{"3", "4"}}},
			},
			want: map[string]any{"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "slice": []any{"12"}}, "scheduler": map[string]any{"image": "ghcr.io/org/repository", "values": []any{"3", "4"}}},
		},
		{
			name: "Test creating none existing paths in base",
			args: args{
				base:   map[string]any{"scheduler": "value"},
				custom: map[string]any{"webserver": map[string]any{"newkey": "value"}},
			},
			want: map[string]any{"scheduler": "value", "webserver": map[string]any{"newkey": "value"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeMaps(tt.args.base, tt.args.custom)
			if !reflect.DeepEqual(tt.args.base, tt.want) {
				t.Errorf("parse() = %v, want %v", tt.args.base, tt.want)
			}
		})
	}
}
