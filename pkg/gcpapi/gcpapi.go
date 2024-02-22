package gcpapi

import (
	"context"
	"fmt"
	"github.com/hashicorp/errwrap"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	"net/http"
	"slices"
)

const (
	ServiceAccountTokenCreatorRole = "roles/iam.serviceAccountTokenCreator"
	WorkloadIdentityUser           = "roles/iam.workloadIdentityUser"
)

type ServiceAccountManager interface {
	Exists(ctx context.Context, name, project string) (bool, error)
}

type ServiceAccountPolicyManager interface {
	GetPolicy(ctx context.Context, resource string) (*iam.Policy, error)
	SetPolicy(ctx context.Context, resource string, policy *iam.Policy) (*iam.Policy, error)
}

type ServiceAccountPolicyBinder interface {
	AddPolicyBinding(ctx context.Context, resource string, policy *iam.Binding) (*iam.Policy, error)
	RemovePolicyRoleBinding(ctx context.Context, resource string, role string) (*iam.Policy, error)
	RemovePolicyRoleMemberBinding(ctx context.Context, resource string, binding *iam.Binding) (*iam.Policy, error)
}

// ServiceAccountResource returns a fully qualified resource name for a service account.
// - https://cloud.google.com/iam/docs/reference/rest/v1/projects.serviceAccounts/getIamPolicy
func ServiceAccountResource(name, project string) string {
	return fmt.Sprintf("projects/%s/serviceAccounts/%s", project, ServiceAccountEmail(name, project))
}

func ServiceAccountEmail(name, project string) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com", name, project)
}

func ServiceAccountEmailMember(name, project string) string {
	return fmt.Sprintf("serviceAccount:%s", ServiceAccountEmail(name, project))
}

func ServiceAccountKubernetesMember(name, namespace, project string) string {
	return fmt.Sprintf("serviceAccount:%s.svc.id.goog[%s/%s]", project, namespace, name)
}

func ServiceAccountTokenCreatorRoleBinding(name, project string) *iam.Binding {
	return NewBinding(ServiceAccountTokenCreatorRole, ServiceAccountEmailMember(name, project))
}

func NewBinding(role, member string) *iam.Binding {
	return &iam.Binding{
		Role:    role,
		Members: []string{member},
	}
}

type serviceAccountPolicyManager struct {
	*iam.Service
}

func (s *serviceAccountPolicyManager) SetPolicy(ctx context.Context, resource string, policy *iam.Policy) (*iam.Policy, error) {
	request := &iam.SetIamPolicyRequest{
		Policy: policy,
	}

	p, err := s.Projects.ServiceAccounts.
		SetIamPolicy(resource, request).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("setting service account policy: %w", err)
	}

	return p, nil
}

func (s *serviceAccountPolicyManager) GetPolicy(ctx context.Context, resource string) (*iam.Policy, error) {
	policy, err := s.Projects.ServiceAccounts.GetIamPolicy(resource).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting service account policy: %w", err)
	}

	return policy, nil
}

func NewServiceAccountPolicyManager(service *iam.Service) ServiceAccountPolicyManager {
	return &serviceAccountPolicyManager{
		Service: service,
	}
}

type serviceAccountPolicyBinder struct {
	manager ServiceAccountPolicyManager
}

func (b *serviceAccountPolicyBinder) AddPolicyBinding(ctx context.Context, resource string, binding *iam.Binding) (*iam.Policy, error) {
	p, err := b.manager.GetPolicy(ctx, resource)
	if err != nil {
		return nil, err
	}

	foundRole := false

	for _, b := range p.Bindings {
		if b.Role == binding.Role {
			foundRole = true

			for _, m := range binding.Members {
				if !slices.Contains(b.Members, m) {
					b.Members = append(b.Members, m)
				}
			}
		}
	}

	if !foundRole {
		p.Bindings = append(p.Bindings, binding)
	}

	return b.manager.SetPolicy(ctx, resource, p)
}

func (b *serviceAccountPolicyBinder) RemovePolicyRoleMemberBinding(ctx context.Context, resource string, binding *iam.Binding) (*iam.Policy, error) {
	p, err := b.manager.GetPolicy(ctx, resource)
	if err != nil {
		return nil, err
	}

	for _, b := range p.Bindings {
		if b.Role == binding.Role {
			for _, m := range binding.Members {
				b.Members = slices.DeleteFunc(b.Members, func(e string) bool {
					return e == m
				})
			}

		}
	}

	return b.manager.SetPolicy(ctx, resource, p)
}

func (b *serviceAccountPolicyBinder) RemovePolicyRoleBinding(ctx context.Context, resource string, role string) (*iam.Policy, error) {
	p, err := b.manager.GetPolicy(ctx, resource)
	if err != nil {
		return nil, err
	}

	p.Bindings = slices.DeleteFunc(p.Bindings, func(e *iam.Binding) bool {
		return e.Role == role
	})

	return b.manager.SetPolicy(ctx, resource, p)
}

func NewServiceAccountPolicyBinder(manager ServiceAccountPolicyManager) ServiceAccountPolicyBinder {
	return &serviceAccountPolicyBinder{
		manager: manager,
	}
}

type serviceAccountManager struct {
	*iam.Service
}

func (s *serviceAccountManager) Exists(ctx context.Context, name, project string) (bool, error) {
	_, err := s.Projects.ServiceAccounts.Get(ServiceAccountResource(name, project)).Context(ctx).Do()
	if err != nil {
		if IsGoogleApiErrorWithCode(err, http.StatusNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("getting service account: %w", err)
	}

	return true, nil
}

func NewServiceAccountManager(service *iam.Service) ServiceAccountManager {
	return &serviceAccountManager{
		Service: service,
	}
}

func NewIAMService(ctx context.Context, c *http.Client) (*iam.Service, error) {
	s, err := iam.NewService(ctx, option.WithHTTPClient(c))
	if err != nil {
		return nil, fmt.Errorf("creating iam service: %w", err)
	}

	return s, nil
}

// Borrowed from Hashicorp's GCP provider:
// - https://github.com/hashicorp/terraform-provider-google/blob/main/google/transport/transport.go#L150-L153
func IsGoogleApiErrorWithCode(err error, errCode int) bool {
	gerr, ok := errwrap.GetType(err, &googleapi.Error{}).(*googleapi.Error)
	return ok && gerr != nil && gerr.Code == errCode
}
