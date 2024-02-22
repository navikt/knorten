package chart

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	gErrors "github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
)

func createBucket(ctx context.Context, teamID, bucketName, gcpProject, gcpRegion string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	buckets := client.Buckets(ctx, gcpProject)
	for {
		b, err := buckets.Next()
		if err != nil {
			if err == iterator.Done {
				break
			}
		}
		if b.Name == bucketName {
			return nil
		}
	}

	storageClassAndLocation := &storage.BucketAttrs{
		StorageClass:             "STANDARD",
		Location:                 gcpRegion,
		UniformBucketLevelAccess: storage.UniformBucketLevelAccess{Enabled: true},
		PublicAccessPrevention:   storage.PublicAccessPreventionEnforced,
		Labels: map[string]string{
			"team":       teamID,
			"created-by": "knorten",
		},
	}

	bucket := client.Bucket(bucketName)

	if err := bucket.Create(ctx, gcpProject, storageClassAndLocation); err != nil {
		apiError, ok := gErrors.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.OK {
				return nil
			}

			if apiError.GRPCStatus().Code() == codes.Unknown {
				if strings.Contains(apiError.GRPCStatus().Message(), "Error 409") {
					// Error 409: Your previous request to create the named bucket succeeded and you already own it., conflict
					return nil
				}
			}
		}

		return err
	}

	return nil
}

func createServiceAccountObjectAdminBinding(ctx context.Context, teamID, bucketName, gcpProject string) error {
	sa := fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject)
	role := iam.RoleName("roles/storage.objectAdmin")

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	handle := client.Bucket(bucketName).IAM()
	policy, err := handle.Policy(ctx)
	if err != nil {
		return err
	}

	members := policy.Members(role)
	for _, m := range members {
		if m == sa {
			return nil
		}
	}

	policy.Add(sa, role)
	if err := handle.SetPolicy(ctx, policy); err != nil {
		return err
	}

	return nil
}
