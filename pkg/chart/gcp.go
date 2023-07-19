package chart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	gErrors "github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/grpc/codes"
)

func removeSQLClientIAMBinding(ctx context.Context, gcpProject, teamID string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"remove-iam-policy-binding",
		gcpProject,
		"--member",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject),
		"--role=roles/cloudsql.client",
		"--condition=None")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
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

func deleteCloudSQLInstance(ctx context.Context, instanceName, gcpProject string) error {
	exisits, err := cloudSQLInstanceExisits(ctx, instanceName, gcpProject)
	if err != nil {
		return err
	}
	if !exisits {
		return nil
	}

	deleteCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"instances",
		"delete",
		instanceName,
		"--project", gcpProject)

	buf := &bytes.Buffer{}
	deleteCmd.Stdout = buf
	deleteCmd.Stderr = os.Stderr
	if err := deleteCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
	}

	return nil
}

func cloudSQLInstanceExisits(ctx context.Context, instanceName, gcpProject string) (bool, error) {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"instances",
		"list",
		"--format=get(name)",
		"--project", gcpProject,
		"--filter", instanceName)

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}

	var instances []string
	if err := json.Unmarshal(buf.Bytes(), &instances); err != nil {
		return false, err
	}

	return len(instances) > 0, nil
}

func createCloudSQLInstance(ctx context.Context, dbInstance, gcpProject, gcpRegion string) error {
	sqlInstances, err := listSQLInstances(gcpProject)
	if err != nil {
		return err
	}

	if contains(sqlInstances, dbInstance) {
		return nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 25*time.Minute)
	cmd := exec.CommandContext(
		ctxWithTimeout,
		"gcloud",
		"sql",
		"instances",
		"create",
		dbInstance,
		fmt.Sprintf("--project=%v", gcpProject),
		fmt.Sprintf("--region=%v", gcpRegion),
		"--database-version=POSTGRES_14",
		"--cpu=1",
		"--memory=3.75GB",
		"--require-ssl",
		"--backup",
		"--backup-start-time=02:00")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		cancel()
		return err
	}

	cancel()
	return nil
}

func createCloudSQLDatabase(ctx context.Context, dbName, dbInstance, gcpProject string) error {
	sqlDBs, err := listSQLDatabases(dbInstance, gcpProject)
	if err != nil {
		return err
	}

	if contains(sqlDBs, dbName) {
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"databases",
		"create",
		dbName,
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", gcpProject))

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
	}

	return nil
}

func createOrUpdateCloudSQLUser(ctx context.Context, user, password, dbInstance, gcpProject string) error {
	sqlUsers, err := listSQLUsers(dbInstance, gcpProject)
	if err != nil {
		return err
	}

	if contains(sqlUsers, user) {
		return updateSQLUser(ctx, user, password, dbInstance, gcpProject)
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"users",
		"create",
		user,
		fmt.Sprintf("--password=%v", password),
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", gcpProject))

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
	}

	return nil
}

func setSQLClientIAMBinding(ctx context.Context, teamID, gcpProject string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"add-iam-policy-binding",
		gcpProject,
		"--member",
		fmt.Sprintf("serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject),
		"--role=roles/cloudsql.client",
		"--condition=None")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
	}

	return nil
}

func listSQLInstances(gcpProject string) ([]map[string]any, error) {
	listCmd := exec.Command(
		"gcloud",
		"sql",
		"instances",
		"list",
		"--format=json",
		fmt.Sprintf("--project=%v", gcpProject))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return nil, err
	}

	var sqlInstances []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sqlInstances); err != nil {
		return nil, err
	}

	return sqlInstances, nil
}

func listSQLDatabases(dbInstance, gcpProject string) ([]map[string]any, error) {
	listCmd := exec.Command(
		"gcloud",
		"sql",
		"databases",
		"list",
		"--format=json",
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", gcpProject))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return nil, err
	}

	var sqlDBs []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sqlDBs); err != nil {
		return nil, err
	}

	return sqlDBs, nil
}

func listSQLUsers(dbInstance, gcpProject string) ([]map[string]any, error) {
	listCmd := exec.Command(
		"gcloud",
		"sql",
		"users",
		"list",
		"--format=json",
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", gcpProject))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return nil, err
	}

	var sqlUsers []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &sqlUsers); err != nil {
		return nil, err
	}
	return sqlUsers, nil
}

func updateSQLUser(ctx context.Context, user, password, dbInstance, gcpProject string) error {
	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"sql",
		"users",
		"set-password",
		user,
		fmt.Sprintf("--password=%v", password),
		fmt.Sprintf("--instance=%v", dbInstance),
		fmt.Sprintf("--project=%v", gcpProject))

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		return err
	}

	return nil
}

func contains(list []map[string]any, itemName string) bool {
	for _, i := range list {
		if i["name"] == itemName {
			return true
		}
	}

	return false
}
