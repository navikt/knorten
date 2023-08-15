package gcp

import (
	"context"

	"cloud.google.com/go/iam"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
	"k8s.io/utils/strings/slices"
)

const secretRoleName = "roles/owner"

func SetUsersSecretOwnerBinding(ctx context.Context, users []string, secret string) error {
	users = addUserTypePrefix(users)

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	handle := client.IAM(secret)
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	policyMembers := policy.Members(secretRoleName)
	for _, member := range policyMembers {
		if !slices.Contains(users, member) {
			policy.Remove(member, secretRoleName)
		}
	}

	err = handle.SetPolicy(ctx, policy)
	if err != nil {
		return err
	}

	for _, user := range users {
		if err := updatePolicy(ctx, handle, user); err != nil {
			return err
		}
	}

	return nil
}

func updatePolicy(ctx context.Context, handle *iam.Handle, user string) error {
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	policyMembers := policy.Members(secretRoleName)
	if !slices.Contains(policyMembers, user) {
		policy.Add(user, secretRoleName)
		err = handle.SetPolicy(ctx, policy)
		if err != nil {
			apiError, ok := apierror.FromError(err)
			if ok && apiError.GRPCStatus().Code() == codes.InvalidArgument {
				return nil
			}

			return err
		}
	}

	return nil
}

func addUserTypePrefix(users []string) []string {
	prefixedUsers := make([]string, len(users))
	for i, u := range users {
		prefixedUsers[i] = "user:" + u
	}

	return prefixedUsers
}
