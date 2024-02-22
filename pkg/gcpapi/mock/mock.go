package mock

import (
	"context"
	"google.golang.org/api/iam/v1"
)

type ServiceAccountPolicyManager struct {
	GetPolicyFunc func(ctx context.Context, name string) (*iam.Policy, error)
	SetPolicyFunc func(ctx context.Context, name string, policy *iam.Policy) (*iam.Policy, error)
}

func (m *ServiceAccountPolicyManager) GetPolicy(ctx context.Context, name string) (*iam.Policy, error) {
	return m.GetPolicyFunc(ctx, name)
}

func (m *ServiceAccountPolicyManager) SetPolicy(ctx context.Context, name string, policy *iam.Policy) (*iam.Policy, error) {
	return m.SetPolicyFunc(ctx, name, policy)
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
