package chart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	gErrors "github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"k8s.io/utils/strings/slices"
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

func removeSQLClientIAMBinding(ctx context.Context, gcpProject, teamID string) error {
	role := "roles/cloudsql.client"
	exists, err := roleBindingExistsInGCP(ctx, gcpProject, teamID, role)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"projects",
		"remove-iam-policy-binding",
		gcpProject,
		fmt.Sprintf("--member=serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject),
		"--role",
		role,
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

func deleteCloudSQLInstance(ctx context.Context, instanceName, gcpProject string) error {
	exisits, err := sqlInstanceExistsInGCP(ctx, instanceName, gcpProject)
	if err != nil {
		return err
	}

	if !exisits {
		return nil
	}

	if err := removeSQLInstanceDeletionProtection(ctx, instanceName, gcpProject); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx,
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

func createCloudSQLInstance(ctx context.Context, teamSlug, dbInstance, gcpProject, gcpRegion string) error {
	exists, err := sqlInstanceExistsInGCP(ctx, dbInstance, gcpProject)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	if exists {
		if err := awaitInstanceOperationsComplete(ctx, dbInstance, gcpProject); err != nil {
			return err
		}

		cmd = exec.CommandContext(ctx,
			"gcloud",
			"--quiet",
			"beta",
			"sql",
			"instances",
			"patch",
			dbInstance,
			"--project", gcpProject,
			"--update-labels", fmt.Sprintf("created-by=knorten,team=%v", teamSlug),
			"--async")
	} else {
		cmd = exec.CommandContext(ctx,
			"gcloud",
			"--quiet",
			"beta",
			"sql",
			"instances",
			"create",
			dbInstance,
			"--project", gcpProject,
			"--region", gcpRegion,
			"--database-version=POSTGRES_14",
			"--deletion-protection",
			"--cpu=1",
			fmt.Sprintf("--labels=created-by=knorten,team=%v", teamSlug),
			"--memory=3.75GB",
			"--require-ssl",
			"--backup",
			"--backup-start-time=02:00")
	}

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

type instanceOperation struct {
	Name string `json:"name"`
}

func awaitInstanceOperationsComplete(ctx context.Context, dbInstance, gcpProject string) error {
	runningOps, err := listRunnningSQLInstanceOperations(ctx, dbInstance, gcpProject)
	if err != nil {
		return err
	}

	waitTimeout := 300
	if deadline, ok := ctx.Deadline(); ok {
		waitTimeout = int(time.Until(deadline).Seconds())
	}

	for _, op := range runningOps {
		cmd := exec.CommandContext(ctx,
			"gcloud",
			"--quiet",
			"--project", gcpProject,
			"sql",
			"operations",
			"wait",
			op.Name,
			fmt.Sprintf("--timeout=%v", waitTimeout),
		)

		stdOut := &bytes.Buffer{}
		stdErr := &bytes.Buffer{}
		cmd.Stdout = stdOut
		cmd.Stderr = stdErr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
		}
	}

	return nil
}

func listRunnningSQLInstanceOperations(ctx context.Context, dbInstance, gcpProject string) ([]*instanceOperation, error) {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"sql",
		"operations",
		"list",
		fmt.Sprintf("--instance=%v", dbInstance),
		"--format=json",
		"--filter='NOT status:done'",
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	instanceOps := []*instanceOperation{}
	if err := json.Unmarshal(stdOut.Bytes(), &instanceOps); err != nil {
		return nil, nil
	}

	return instanceOps, nil
}

func createCloudSQLDatabase(ctx context.Context, dbName, dbInstance, gcpProject string) error {
	exists, err := sqlDatabaseExistsInGCP(ctx, dbInstance, gcpProject, dbName)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	cmd := exec.CommandContext(ctx,
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

func createOrUpdateCloudSQLUser(ctx context.Context, user, password, dbInstance, gcpProject string) error {
	exists, err := sqlUserExistsInGCP(ctx, dbInstance, gcpProject, user)
	if err != nil {
		return err
	}

	if exists {
		return updateSQLUser(ctx, user, password, dbInstance, gcpProject)
	}

	return createSQLUser(ctx, user, password, dbInstance, gcpProject)
}

func updateSQLUser(ctx context.Context, user, password, dbInstance, gcpProject string) error {
	if err := awaitInstanceOperationsComplete(ctx, dbInstance, gcpProject); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx,
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

func createSQLUser(ctx context.Context, user, password, dbInstance, gcpProject string) error {
	cmd := exec.CommandContext(ctx,
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

func setSQLClientIAMBinding(ctx context.Context, teamID, gcpProject string) error {
	role := "roles/cloudsql.client"
	exists, err := roleBindingExistsInGCP(ctx, gcpProject, teamID, role)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"projects",
		"add-iam-policy-binding",
		gcpProject,
		fmt.Sprintf("--role=%v", role),
		"--condition=None",
		fmt.Sprintf("--member=serviceAccount:%v@%v.iam.gserviceaccount.com", teamID, gcpProject))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func sqlInstanceExistsInGCP(ctx context.Context, instanceName, gcpProject string) (bool, error) {
	cmd := exec.CommandContext(ctx,
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

func removeSQLInstanceDeletionProtection(ctx context.Context, instanceName, gcpProject string) error {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"sql",
		"instances",
		"patch",
		instanceName,
		"--project", gcpProject,
		"--no-deletion-protection")

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return nil
}

func sqlDatabaseExistsInGCP(ctx context.Context, dbInstance, gcpProject, sqlDatabase string) (bool, error) {
	cmd := exec.CommandContext(ctx,
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

func sqlUserExistsInGCP(ctx context.Context, dbInstance, gcpProject, sqlUser string) (bool, error) {
	cmd := exec.CommandContext(ctx,
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

func roleBindingExistsInGCP(ctx context.Context, gcpProject, teamID, role string) (bool, error) {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"projects",
		"get-iam-policy",
		gcpProject,
		"--format=get(bindings.role)",
		"--flatten=bindings[].members",
		fmt.Sprintf("--filter=bindings.members:%v@%v.iam.gserviceaccount.com AND bindings.members!~deleted:", teamID, gcpProject))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	roles := strings.Split(stdOut.String(), "\n")
	return slices.Contains(roles, role), nil
}

func grantSATokenCreatorRole(ctx context.Context, teamID, gcpProject string) error {
	role := "roles/iam.serviceAccountTokenCreator"

	sa := fmt.Sprintf("%v@%v.iam.gserviceaccount.com", teamID, gcpProject)

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"iam",
		"service-accounts",
		"add-iam-policy-binding",
		sa,
		fmt.Sprintf("--role=%v", role),
		fmt.Sprintf("--member=serviceAccount:%v", sa),
		"--quiet",
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}
	return nil
}

func deleteTokenCreatorRoleOnSA(ctx context.Context, teamID, gcpProject string) error {
	if exist, err := serviceAccountExistsInGCP(ctx, teamID, gcpProject); err != nil {
		return err
	} else if !exist {
		return nil
	}

	role := "roles/iam.serviceAccountTokenCreator"

	sa := fmt.Sprintf("%v@%v.iam.gserviceaccount.com", teamID, gcpProject)

	cmd := exec.CommandContext(ctx,
		"gcloud",
		"iam",
		"service-accounts",
		"remove-iam-policy-binding",
		sa,
		fmt.Sprintf("--role=%v", role),
		fmt.Sprintf("--member=serviceAccount:%v", sa),
		"--quiet",
	)

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}
	return nil
}

func serviceAccountExistsInGCP(ctx context.Context, teamID, gcpProject string) (bool, error) {
	cmd := exec.CommandContext(ctx,
		"gcloud",
		"--quiet",
		"iam",
		"service-accounts",
		"list",
		"--format=get(email)",
		"--project", gcpProject,
		fmt.Sprintf("--filter=email=%v@%v.iam.gserviceaccount.com", teamID, gcpProject))

	stdOut := &bytes.Buffer{}
	stdErr := &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\nstderr: %v", err, stdErr.String())
	}

	return stdOut.String() != "", nil
}
