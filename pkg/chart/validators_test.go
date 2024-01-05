package chart

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func TestValidateCPUSpec(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "single cpu core",
			input:     "1",
			wantError: false,
		},
		{
			name:      "multiple cpu cores",
			input:     "10",
			wantError: false,
		},
		{
			name:      "cpu fractions",
			input:     "1.5",
			wantError: false,
		},
		{
			name:      "millicores",
			input:     "100m",
			wantError: false,
		},
		{
			name:      "invalid millicore",
			input:     "1.5m",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := binding.Validator.Engine().(*validator.Validate)
			err := v.RegisterValidation("validCPUSpec", ValidateCPUSpec)
			if err != nil {
				t.Errorf("Cannot register validator: %v", err)
			}
			got := v.Struct(struct {
				Name string `form:"cpuspec" binding:"validCPUSpec"`
			}{
				Name: tc.input,
			})
			if (got != nil) != tc.wantError {
				t.Errorf("got != want; %v != %v", got, tc.wantError)
			}
		})
	}
}

func TestValidateMemorySpec(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "bytes",
			input:     "100000",
			wantError: false,
		},
		{
			name:      "M",
			input:     "1000M",
			wantError: false,
		},
		{
			name:      "Mi",
			input:     "1000Mi",
			wantError: false,
		},
		{
			name:      "G",
			input:     "10G",
			wantError: false,
		},
		{
			name:      "Gi",
			input:     "1Gi",
			wantError: false,
		},
		{
			name:      "invalid bytes",
			input:     "1gig",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := binding.Validator.Engine().(*validator.Validate)
			err := v.RegisterValidation("validMemorySpec", ValidateMemorySpec)
			if err != nil {
				t.Errorf("Cannot register validator: %v", err)
			}
			got := v.Struct(struct {
				Name string `form:"memoryspec" binding:"validMemorySpec"`
			}{
				Name: tc.input,
			})
			if (got != nil) != tc.wantError {
				t.Errorf("got != want; %v != %v", got, tc.wantError)
			}
		})
	}
}
