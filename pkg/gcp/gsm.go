package gcp

import (
	"context"
	"encoding/json"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
)

func ListSecrets(ctx context.Context, teamID, gcpProject, gcpRegion, filter string) ([]*secretmanagerpb.Secret, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	secretsIterator := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%v", gcpProject),
		Filter: filter,
	})

	secrets := []*secretmanagerpb.Secret{}
	for {
		secret, err := secretsIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		secrets = append(secrets, secret)
	}

	return secrets, nil
}

func GetSecret(ctx context.Context, secretName string) (*secretmanagerpb.Secret, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
		Name: secretName,
	})
}

func GetLatestSecretVersion(ctx context.Context, secretName string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("%v/versions/latest", secretName),
	})
}

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

func AddSecretVersion(ctx context.Context, gcpProject, location, secretName, secretValue string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	secretBytes, err := json.Marshal(secretValue)
	if err != nil {
		return err
	}

	_, err = client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: fmt.Sprintf("projects/%v/locations/%v/secrets/%v", gcpProject, location, secretName),
		Payload: &secretmanagerpb.SecretPayload{
			Data: secretBytes,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func DeleteSecret(ctx context.Context, secretID string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	req := &secretmanagerpb.DeleteSecretRequest{
		Name: secretID,
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
