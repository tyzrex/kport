package cli

import (
	"reflect"
	"testing"
)

func TestParsePortArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []int
		wantErr bool
	}{
		{
			name:    "single port",
			args:    []string{"3000"},
			want:    []int{3000},
			wantErr: false,
		},
		{
			name:    "colon prefixed port",
			args:    []string{":8080"},
			want:    []int{8080},
			wantErr: false,
		},
		{
			name:    "preset name next",
			args:    []string{"next"},
			want:    []int{3000},
			wantErr: false,
		},
		{
			name:    "preset name vite and postgres",
			args:    []string{"vite", "postgres"},
			want:    []int{5173, 5432},
			wantErr: false,
		},
		{
			name:    "multiple ports and presets",
			args:    []string{"3000", "8080", "5432"},
			want:    []int{3000, 8080, 5432},
			wantErr: false,
		},
		{
			name:    "comma separated ports and presets",
			args:    []string{"next,8080,redis"},
			want:    []int{3000, 8080, 6379},
			wantErr: false,
		},
		{
			name:    "port range",
			args:    []string{"3000-3003"},
			want:    []int{3000, 3001, 3002, 3003},
			wantErr: false,
		},
		{
			name:    "mixed formats with duplicates",
			args:    []string{":3000", "3000-3002", "8080,3001"},
			want:    []int{3000, 3001, 3002, 8080},
			wantErr: false,
		},
		{
			name:    "invalid port string",
			args:    []string{"unknownpreset123"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "port out of range",
			args:    []string{"70000"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid range reverse",
			args:    []string{"5000-3000"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePortArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePortArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "flags before port",
			args: []string{"-d", "3000"},
			want: []string{"-d", "3000"},
		},
		{
			name: "flags after port",
			args: []string{"next", "-d"},
			want: []string{"-d", "next"},
		},
		{
			name: "flag with value after port",
			args: []string{"3000", "-p", "tcp", "-f"},
			want: []string{"-p", "tcp", "-f", "3000"},
		},
		{
			name: "mixed with ranges and presets",
			args: []string{"3000-3005", "-T", "postgres", "--inspect"},
			want: []string{"-T", "--inspect", "3000-3005", "postgres"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
