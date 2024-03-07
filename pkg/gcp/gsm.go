package gcp

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
)

func CreateSecret(ctx context.Context, gcpProject, gcpRegion, secretID string, labels map[string]string) (*secretmanagerpb.Secret, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	labels["created-by"] = "knorten"

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/" + gcpProject,
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Labels: labels,
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_UserManaged_{
					UserManaged: &secretmanagerpb.Replication_UserManaged{
						Replicas: []*secretmanagerpb.Replication_UserManaged_Replica{
							{
								Location: gcpRegion,
							},
						},
					},
				},
			},
		},
	}

	s, err := client.CreateSecret(ctx, req)
	if err != nil {
		apiError, ok := apierror.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.AlreadyExists {
				return client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
					Name: fmt.Sprintf("projects/%v/secrets/%v", gcpProject, secretID),
				})
			}
		}
		return nil, err
	}

	return s, nil
}

func DeleteSecret(ctx context.Context, gcpProject, secretID string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	project := fmt.Sprintf("projects/%v", gcpProject)
	_ = client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent:   project,
		PageSize: int32(500),
	})

	req := &secretmanagerpb.DeleteSecretRequest{
		Name: fmt.Sprintf("%v/secrets/%v", project, secretID),
	}

	err = client.DeleteSecret(ctx, req)
	if err != nil {
		apiError, ok := apierror.FromError(err)
		if ok && apiError.GRPCStatus().Code() == codes.NotFound {
			return nil
		}

		return err
	}

	return nil
}
