package gcpapi_test

import (
	"context"
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/jarcoal/httpmock"
	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/gcpapi/mock"
	"github.com/navikt/knorten/pkg/testutils"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"net/http"
	"testing"
)

const (
	notFoundError = `{
  "error": {
    "code": 404,
    "message": "Unknown service account",
    "status": "NOT_FOUND"
  }
}`
)

func echoResponder(t *testing.T) httpmock.Responder {
	t.Helper()

	return func(request *http.Request) (*http.Response, error) {
		var r *iam.SetIamPolicyRequest

		err := json.NewDecoder(request.Body).Decode(&r)
		if err != nil {
			t.Fatalf("unexpected request body: %v", err)
		}

		data, err := json.Marshal(r.Policy)
		if err != nil {
			t.Fatalf("failed to marshal policy: %v", err)
		}

		return httpmock.NewBytesResponse(http.StatusOK, data), nil
	}
}

func mustService(t *testing.T) *iam.Service {
	t.Helper()

	s, err := gcpapi.NewIAMService(context.Background(), testutils.NewLoggingHttpClient())
	if err != nil {
		t.Fatalf("creating IAM service: %v", err)
	}

	return s
}

func TestServiceAccountPolicyManager_GetPolicy(t *testing.T) {
	name, project := "fake-sa-name", "fake-gcp-project"

	testCases := []struct {
		name               string
		serviceAccountName string
		method             string
		url                string
		responder          httpmock.Responder
		expectErr          bool
		expect             any
	}{
		{
			name:               "Should get policy",
			serviceAccountName: name,
			method:             http.MethodPost,
			url:                "https://iam.googleapis.com/v1/projects/fake-gcp-project/serviceAccounts/fake-sa-name@fake-gcp-project.iam.gserviceaccount.com:getIamPolicy?alt=json&prettyPrint=false",
			responder: httpmock.NewJsonResponderOrPanic(http.StatusOK, &iam.Policy{
				Bindings: []*iam.Binding{
					gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
				},
			}),
			expectErr: false,
			expect: &iam.Policy{
				Bindings: []*iam.Binding{
					gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
				},
				ServerResponse: googleapi.ServerResponse{
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					HTTPStatusCode: http.StatusOK,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(tc.method, tc.url, tc.responder)

			var got any

			got, err := gcpapi.NewServiceAccountPolicyManager(project, mustService(t)).GetPolicy(context.Background(), tc.serviceAccountName)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				got = err.Error()
			}

			diff := cmp.Diff(tc.expect, got)
			if diff != "" {
				t.Fatalf("unexpected policy:\n\n%s\n", diff)
			}

		})
	}
}

func TestServiceAccountPolicyManager_SetPolicy(t *testing.T) {
	name, project := "fake-sa-name", "fake-gcp-project"

	testCases := []struct {
		name               string
		serviceAccountName string
		policy             *iam.Policy
		method             string
		url                string
		responder          httpmock.Responder
		expectErr          bool
		expect             any
	}{
		{
			name:               "Should set policy",
			serviceAccountName: name,
			policy: &iam.Policy{
				Bindings: []*iam.Binding{
					gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
				},
			},
			method:    http.MethodPost,
			url:       "https://iam.googleapis.com/v1/projects/fake-gcp-project/serviceAccounts/fake-sa-name@fake-gcp-project.iam.gserviceaccount.com:setIamPolicy?alt=json&prettyPrint=false",
			responder: echoResponder(t),
			expectErr: false,
			expect: &iam.Policy{
				Bindings: []*iam.Binding{
					gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
				},
				ServerResponse: googleapi.ServerResponse{
					Header:         http.Header{},
					HTTPStatusCode: http.StatusOK,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(tc.method, tc.url, tc.responder)

			got, err := gcpapi.NewServiceAccountPolicyManager(project, mustService(t)).SetPolicy(context.Background(), tc.serviceAccountName, tc.policy)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				diff := cmp.Diff(tc.expect, got)
				if diff != "" {
					t.Fatalf("unexpected policy:\n\n%s\n", diff)
				}
			}

		})
	}
}

func TestServiceAccountPolicyBinder_AddPolicyBinding(t *testing.T) {
	name, project := "fake-sa-name", "fake-gcp-project"

	testCases := []struct {
		name   string
		role   gcpapi.ServiceAccountRole
		binder gcpapi.ServiceAccountPolicyBinder
		expect *iam.Policy
	}{
		{
			name: "Should add policy binding to empty policy",
			role: gcpapi.ServiceAccountTokenCreatorRole,
			binder: gcpapi.NewServiceAccountPolicyBinder(project, mock.NewServiceAccountPolicyManager(
				&iam.Policy{}, nil,
			)),
			expect: &iam.Policy{
				Bindings: []*iam.Binding{
					gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
				},
			},
		},
		{
			name: "Should add member to existing binding",
			role: gcpapi.ServiceAccountTokenCreatorRole,
			binder: gcpapi.NewServiceAccountPolicyBinder(project, mock.NewServiceAccountPolicyManager(
				&iam.Policy{
					Bindings: []*iam.Binding{
						{
							Role:    gcpapi.ServiceAccountTokenCreatorRole.String(),
							Members: []string{"serviceAccount:something"},
						},
					},
				},
				nil,
			)),
			expect: &iam.Policy{
				Bindings: []*iam.Binding{
					{
						Role:    gcpapi.ServiceAccountTokenCreatorRole.String(),
						Members: []string{"serviceAccount:something", gcpapi.ServiceAccountEmailMember(name, project)},
					},
				},
			},
		},
		{
			name: "Should ignore existing member",
			role: gcpapi.ServiceAccountTokenCreatorRole,
			binder: gcpapi.NewServiceAccountPolicyBinder(project, mock.NewServiceAccountPolicyManager(
				&iam.Policy{
					Bindings: []*iam.Binding{
						gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
					},
				},
				nil,
			)),
			expect: &iam.Policy{
				Bindings: []*iam.Binding{
					gcpapi.ServiceAccountTokenCreatorRoleBinding(name, project),
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.binder.AddPolicyRole(context.Background(), name, tc.role)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			diff := cmp.Diff(tc.expect, got)
			if diff != "" {
				t.Fatalf("unexpected policy:\n\n%s\n", diff)
			}
		})
	}
}

func TestServiceAccountPolicyBinder_RemovePolicyRoleBinding(t *testing.T) {
	name, project := "fake-sa-name", "fake-gcp-project"

	testCases := []struct {
		name   string
		role   gcpapi.ServiceAccountRole
		binder gcpapi.ServiceAccountPolicyBinder
		expect *iam.Policy
	}{
		{
			name: "Should remove role binding from empty policy",
			role: gcpapi.ServiceAccountTokenCreatorRole,
			binder: gcpapi.NewServiceAccountPolicyBinder(project, mock.NewServiceAccountPolicyManager(
				&iam.Policy{}, nil,
			)),
			expect: &iam.Policy{},
		},
		{
			name: "Should remove member from existing binding",
			role: gcpapi.ServiceAccountTokenCreatorRole,
			binder: gcpapi.NewServiceAccountPolicyBinder(project, mock.NewServiceAccountPolicyManager(
				&iam.Policy{
					Bindings: []*iam.Binding{
						{
							Role:    gcpapi.ServiceAccountTokenCreatorRole.String(),
							Members: []string{gcpapi.ServiceAccountEmailMember(name, project)},
						},
					},
				},
				nil,
			)),
			expect: &iam.Policy{
				Bindings: []*iam.Binding{},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.binder.RemovePolicyRole(context.Background(), name, tc.role)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			diff := cmp.Diff(tc.expect, got)
			if diff != "" {
				t.Fatalf("unexpected policy:\n\n%s\n", diff)
			}
		})
	}
}

func TestServiceAccountManager_Exists(t *testing.T) {
	name, project := "fake-sa-name", "fake-gcp-project"

	testCases := []struct {
		name      string
		method    string
		url       string
		responder httpmock.Responder
		expectErr bool
		expect    any
	}{
		{
			name:      "Should return false if service account doesn't exist",
			method:    http.MethodGet,
			url:       "https://iam.googleapis.com/v1/projects/fake-gcp-project/serviceAccounts/fake-sa-name@fake-gcp-project.iam.gserviceaccount.com?alt=json&prettyPrint=false",
			responder: httpmock.NewStringResponder(http.StatusNotFound, notFoundError),
			expect:    false,
		},
		{
			name:      "Should return true if service account exists",
			method:    http.MethodGet,
			url:       "https://iam.googleapis.com/v1/projects/fake-gcp-project/serviceAccounts/fake-sa-name@fake-gcp-project.iam.gserviceaccount.com?alt=json&prettyPrint=false",
			responder: httpmock.NewJsonResponderOrPanic(http.StatusOK, &iam.ServiceAccount{}),
			expect:    true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			httpmock.RegisterResponder(tc.method, tc.url, tc.responder)

			got, err := gcpapi.NewServiceAccountManager(project, mustService(t)).Exists(context.Background(), name)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				if err.Error() != tc.expect {
					t.Fatalf("unexpected error: got %v, want %v", err, tc.expect)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if got != tc.expect {
					t.Fatalf("unexpected result: got %v, want %v", got, tc.expect)
				}
			}
		})
	}
}
