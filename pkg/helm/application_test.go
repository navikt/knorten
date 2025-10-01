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
				key:   "webserver.name",
				value: "flowtheair",
				values: map[string]any{
					"webserver": map[string]any{"image": "ghcr.io/org/repo:tag"},
				},
			},
			want: map[string]any{
				"webserver": map[string]any{"image": "ghcr.io/org/repo:tag", "name": "flowtheair"},
			},
		},
		{
			name: "Quoted value",
			args: args{
				key:    "ingress.web.path",
				value:  `"/*"`,
				values: map[string]any{"ingress": map[string]any{"web": map[string]any{}}},
			},
			want: map[string]any{"ingress": map[string]any{"web": map[string]any{"path": "/*"}}},
		},
		{
			name: "Quoted keys and value",
			args: args{
				key:    "ingress.web.annotations.kubernetes\\.io/ingress\\.allow-http",
				value:  `"true"`,
				values: map[string]any{},
			},
			want: map[string]any{
				"ingress": map[string]any{
					"web": map[string]any{
						"annotations": map[string]any{"kubernetes.io/ingress.allow-http": "true"},
					},
				},
			},
		},
		{
			name: "Handle omitted values",
			args: args{
				key:    "fernetKey,omit",
				value:  "secret-password",
				values: map[string]any{},
			},
			want: map[string]any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTeamValue(tt.args.key, tt.args.value, tt.args.values)
			if err != nil {
				t.Error(err)
			}
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
				base: map[string]any{
					"webserver": map[string]any{"image": "ghcr.io/org/repo:tag"},
				},
				custom: map[string]any{
					"webserver": map[string]any{
						"image": "ghcr.io/org/repo:tag2",
						"name":  "flowtheair",
					},
				},
			},
			want: map[string]any{
				"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "name": "flowtheair"},
			},
		},
		{
			name: "With slice",
			args: args{
				base: map[string]any{
					"webserver": map[string]any{
						"image":     "ghcr.io/org/repo:tag",
						"myslice":   []any{"one", "two"},
						"scheduler": "1234",
					},
				},
				custom: map[string]any{
					"webserver": map[string]any{
						"image": "ghcr.io/org/repo:tag2",
						"name":  "flowtheair",
					},
				},
			},
			want: map[string]any{
				"webserver": map[string]any{
					"image":     "ghcr.io/org/repo:tag2",
					"name":      "flowtheair",
					"myslice":   []any{"one", "two"},
					"scheduler": "1234",
				},
			},
		},
		{
			name: "Nested test",
			args: args{
				base: map[string]any{
					"webserver": map[string]any{"image": "ghcr.io/org/repo:tag"},
					"scheduler": map[string]any{
						"image":  "ghcr.io/org/repository",
						"values": []any{"1", "2"},
					},
				},
				custom: map[string]any{
					"webserver": map[string]any{
						"image": "ghcr.io/org/repo:tag2",
						"slice": []any{"12"},
					},
					"scheduler": map[string]any{"values": []any{"3", "4"}},
				},
			},
			want: map[string]any{
				"webserver": map[string]any{"image": "ghcr.io/org/repo:tag2", "slice": []any{"12"}},
				"scheduler": map[string]any{
					"image":  "ghcr.io/org/repository",
					"values": []any{"3", "4"},
				},
			},
		},
		{
			name: "Test creating none existing paths in base",
			args: args{
				base:   map[string]any{"scheduler": "value"},
				custom: map[string]any{"webserver": map[string]any{"newkey": "value"}},
			},
			want: map[string]any{
				"scheduler": "value",
				"webserver": map[string]any{"newkey": "value"},
			},
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

func Test_parseKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantKey  string
		wantOpts []string
	}{
		{
			name:     "Key without options",
			key:      "someHelmKey",
			wantKey:  "someHelmKey",
			wantOpts: []string{},
		},
		{
			name:     "Key with options",
			key:      "noKey,omit",
			wantKey:  "noKey",
			wantOpts: []string{"omit"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, opts := parseKey(tt.key)
			if key != tt.wantKey {
				t.Errorf("parseKey() key = %v, want %v", key, tt.wantKey)
			}
			if !reflect.DeepEqual(opts, tt.wantOpts) {
				t.Errorf("parseKey() opts = %v, want %v", opts, tt.wantOpts)
			}
		})
	}
}
