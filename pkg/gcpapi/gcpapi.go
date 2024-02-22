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

type ServiceAccountRole string

func (r ServiceAccountRole) String() string {
	return string(r)
}

const (
	ServiceAccountTokenCreatorRole ServiceAccountRole = "roles/iam.serviceAccountTokenCreator"
	WorkloadIdentityUser           ServiceAccountRole = "roles/iam.workloadIdentityUser"
)

type ServiceAccountChecker interface {
	Exists(ctx context.Context, name string) (bool, error)
}

type ServiceAccountFetcher interface {
	Get(ctx context.Context, name string) (*iam.ServiceAccount, error)
}

type ServiceAccountPolicyManager interface {
	GetPolicy(ctx context.Context, resource string) (*iam.Policy, error)
	SetPolicy(ctx context.Context, resource string, policy *iam.Policy) (*iam.Policy, error)
}

type ServiceAccountPolicyBinder interface {
	AddPolicyRole(ctx context.Context, name string, role ServiceAccountRole) (*iam.Policy, error)
	RemovePolicyRole(ctx context.Context, name string, role ServiceAccountRole) (*iam.Policy, error)
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
	return NewBinding(ServiceAccountTokenCreatorRole.String(), ServiceAccountEmailMember(name, project))
}

func NewBinding(role, member string) *iam.Binding {
	return &iam.Binding{
		Role:    role,
		Members: []string{member},
	}
}

type serviceAccountPolicyManager struct {
	*iam.Service
	project string
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

func NewServiceAccountPolicyManager(project string, service *iam.Service) ServiceAccountPolicyManager {
	return &serviceAccountPolicyManager{
		Service: service,
		project: project,
	}
}

type serviceAccountPolicyBinder struct {
	manager ServiceAccountPolicyManager
	project string
}

func (b *serviceAccountPolicyBinder) AddPolicyRole(ctx context.Context, name string, role ServiceAccountRole) (*iam.Policy, error) {
	resource := ServiceAccountResource(name, b.project)
	member := ServiceAccountEmailMember(name, b.project)

	p, err := b.manager.GetPolicy(ctx, resource)
	if err != nil {
		return nil, err
	}

	foundRole := false

	for _, b := range p.Bindings {
		if b.Role == role.String() {
			foundRole = true

			if !slices.Contains(b.Members, member) {
				b.Members = append(b.Members, member)
			}
		}
	}

	if !foundRole {
		p.Bindings = append(p.Bindings, NewBinding(role.String(), member))
	}

	return b.manager.SetPolicy(ctx, resource, p)
}

func (b *serviceAccountPolicyBinder) RemovePolicyRole(ctx context.Context, name string, role ServiceAccountRole) (*iam.Policy, error) {
	resource := ServiceAccountResource(name, b.project)

	p, err := b.manager.GetPolicy(ctx, resource)
	if err != nil {
		return nil, err
	}

	p.Bindings = slices.DeleteFunc(p.Bindings, func(e *iam.Binding) bool {
		return e.Role == role.String()
	})

	return b.manager.SetPolicy(ctx, resource, p)
}

func NewServiceAccountPolicyBinder(project string, manager ServiceAccountPolicyManager) ServiceAccountPolicyBinder {
	return &serviceAccountPolicyBinder{
		manager: manager,
		project: project,
	}
}

type serviceAccountFetcher struct {
	*iam.Service
	project string
}

func (s *serviceAccountFetcher) Get(ctx context.Context, name string) (*iam.ServiceAccount, error) {
	resource := ServiceAccountResource(name, s.project)

	sa, err := s.Projects.ServiceAccounts.Get(resource).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return sa, nil
}

func NewServiceAccountFetcher(project string, service *iam.Service) ServiceAccountFetcher {
	return &serviceAccountFetcher{
		Service: service,
		project: project,
	}
}

type serviceAccountChecker struct {
	fetcher ServiceAccountFetcher
	project string
}

func (s *serviceAccountChecker) Exists(ctx context.Context, name string) (bool, error) {
	_, err := s.fetcher.Get(ctx, name)
	if err != nil {
		if IsGoogleApiErrorWithCode(err, http.StatusNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("getting service account: %w", err)
	}

	return true, nil
}

func NewServiceAccountChecker(project string, fetcher ServiceAccountFetcher) ServiceAccountChecker {
	return &serviceAccountChecker{
		fetcher: fetcher,
		project: project,
	}
}

func NewIAMService(ctx context.Context, c *http.Client, project string) (*iam.Service, error) {
	s, err := iam.NewService(ctx, option.WithHTTPClient(c), option.WithQuotaProject(project))
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
