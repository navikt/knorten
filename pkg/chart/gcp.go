package chart

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	gErrors "github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
)

func removeSQLClientIAMBinding(gcpProject, teamID string) error {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"projects",
		"remove-iam-policy-binding",
		gcpProject,
		fmt.Sprintf("--member=serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject),
		"--role=roles/cloudsql.client",
		"--condition=None")

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func createBucket(ctx context.Context, teamID, bucketName, gcpProject, gcpRegion string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

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

func deleteCloudSQLInstance(instanceName, gcpProject string) error {
	exisits, err := sqlInstanceExistsInGCP(instanceName, gcpProject)
	if err != nil {
		return err
	}

	if !exisits {
		return nil
	}

	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"instances",
		"delete",
		instanceName,
		"--project", gcpProject)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func createCloudSQLInstance(dbInstance, gcpProject, gcpRegion string) error {
	exists, err := sqlInstanceExistsInGCP(dbInstance, gcpProject)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"instances",
		"create",
		dbInstance,
		"--project", gcpProject,
		"--region", gcpRegion,
		"--database-version=POSTGRES_14",
		"--cpu=1",
		"--memory=3.75GB",
		"--require-ssl",
		"--backup",
		"--backup-start-time=02:00")

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func createCloudSQLDatabase(dbName, dbInstance, gcpProject string) error {
	exists, err := sqlDatabaseExistsInGCP(dbInstance, gcpProject, dbName)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"databases",
		"create",
		dbName,
		"--instance", dbInstance,
		"--project", gcpProject)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func createOrUpdateCloudSQLUser(user, password, dbInstance, gcpProject string) error {
	exists, err := sqlUserExistsInGCP(dbInstance, gcpProject, user)
	if err != nil {
		return err
	}

	if exists {
		return updateSQLUser(user, password, dbInstance, gcpProject)
	}

	return createSQLUser(user, password, dbInstance, gcpProject)
}

func createSQLUser(user, password, dbInstance, gcpProject string) error {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"users",
		"create",
		user,
		"--password", password,
		"--instance", dbInstance,
		"--project", gcpProject)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func setSQLClientIAMBinding(teamID, gcpProject string) error {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"projects",
		"add-iam-policy-binding",
		gcpProject,
		"--member",
		"--role=roles/cloudsql.client",
		"--condition=None",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func sqlInstanceExistsInGCP(instanceName, gcpProject string) (bool, error) {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"instances",
		"list",
		"--format=get(name)",
		"--project", gcpProject,
		fmt.Sprintf("--filter=name=%v", instanceName))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return stdOut.String() != "", nil
}

func sqlDatabaseExistsInGCP(dbInstance, gcpProject, sqlDatabase string) (bool, error) {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"databases",
		"list",
		"--format=get(name)",
		"--instance", dbInstance,
		"--project", gcpProject,
		fmt.Sprintf("--filter=name=%v", sqlDatabase))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return stdOut.String() != "", nil
}

func sqlUserExistsInGCP(dbInstance, gcpProject, sqlUser string) (bool, error) {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"users",
		"list",
		"--format=get(name)",
		"--instance", dbInstance,
		"--project", gcpProject,
		fmt.Sprintf("--filter=name=%v", sqlUser))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return stdOut.String() != "", nil
}

func updateSQLUser(user, password, dbInstance, gcpProject string) error {
	cmd := exec.Command(
		"gcloud",
		"--quiet",
		"sql",
		"users",
		"set-password",
		user,
		"--instance", dbInstance,
		"--project", gcpProject,
		"--password", password)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}
