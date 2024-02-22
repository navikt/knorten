package mock

import (
	"context"
	"google.golang.org/api/iam/v1"
)

type ServiceAccountPolicyManager struct {
	GetPolicyFunc func(ctx context.Context, resource string) (*iam.Policy, error)
	SetPolicyFunc func(ctx context.Context, resource string, policy *iam.Policy) (*iam.Policy, error)
}

func (m *ServiceAccountPolicyManager) GetPolicy(ctx context.Context, resource string) (*iam.Policy, error) {
	return m.GetPolicyFunc(ctx, resource)
}

func (m *ServiceAccountPolicyManager) SetPolicy(ctx context.Context, resource string, policy *iam.Policy) (*iam.Policy, error) {
	return m.SetPolicyFunc(ctx, resource, policy)
}

func NewServiceAccountPolicyManager(policy *iam.Policy, err error) *ServiceAccountPolicyManager {
	return &ServiceAccountPolicyManager{
		GetPolicyFunc: func(ctx context.Context, resource string) (*iam.Policy, error) {
			return policy, err
		},
		SetPolicyFunc: func(ctx context.Context, resource string, policy *iam.Policy) (*iam.Policy, error) {
			return policy, err
		},
	}
}
