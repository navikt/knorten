package google

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	gErrors "github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
)

func (g *Google) CreateBucket(ctx context.Context, teamID, bucketName string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	storageClassAndLocation := &storage.BucketAttrs{
		StorageClass:             "STANDARD",
		Location:                 g.region,
		UniformBucketLevelAccess: storage.UniformBucketLevelAccess{Enabled: true},
		PublicAccessPrevention:   storage.PublicAccessPreventionEnforced,
		Labels: map[string]string{
			"team":       teamID,
			"created-by": "knorten",
		},
	}

	bucket := client.Bucket(bucketName)

	if err := bucket.Create(ctx, g.project, storageClassAndLocation); err != nil {
		apiError, ok := gErrors.FromError(err)
		if ok {
			if apiError.GRPCStatus().Code() == codes.OK {
				g.log.Infof("create bucket: bucket %v already exists", bucketName)
				return nil
			}
		}
		return err
	}

	return nil
}

func (g *Google) CreateServiceAccountObjectAdminBinding(ctx context.Context, teamID, bucketName string) error {
	sa := fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, g.project)
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
