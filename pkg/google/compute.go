package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	ownerRole     = "roles/owner"
	computeZone   = "europe-west1-b"
	knadaUserRole = "projects/knada-gcp/roles/knadauser"
)

type ComputeForm struct {
	Name        string `form:"name"`
	MachineType string `form:"machine_type"`
}

type computeInstance struct {
	Name string `json:"name"`
}

func (g *Google) CreateComputeInstance(c *gin.Context, slug string) error {
	var form ComputeForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	team, err := g.repo.TeamGet(c, slug)
	if err != nil {
		return err
	}

	computeInstance := TeamToComputeInstanceName(team.ID)

	go g.createComputeInstance(c, team.Users, slug, computeInstance, form.MachineType)

	if err := g.repo.ComputeInstanceCreate(c, team.ID, computeInstance, form.MachineType); err != nil {
		return err
	}

	return nil
}

func (g *Google) UpdateComputeInstanceOwners(ctx context.Context, instance, slug string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	team, err := g.repo.TeamGet(ctx, slug)
	if err != nil {
		return err
	}

	type iamPolicy struct {
		Bindings []struct {
			Members []string `json:"members"`
			Role    string   `json:"role"`
		} `json:"bindings"`
	}

	listCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"get-iam-policy",
		instance,
		fmt.Sprintf("--zone=%v", computeZone),
		"--format=json",
	)

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Errorf("list iam policies for compute instance %v", instance)
		return err
	}

	iam := &iamPolicy{}
	if err := json.Unmarshal(buf.Bytes(), iam); err != nil {
		return err
	}

	currentOwners := []string{}
	for _, b := range iam.Bindings {
		if b.Role == ownerRole {
			currentOwners = b.Members
		}
	}

	if err := g.updateOwners(ctx, instance, currentOwners, team.Users); err != nil {
		return err
	}

	return nil
}

func (g *Google) DeleteComputeInstance(ctx context.Context, slug string) error {
	team, err := g.repo.TeamGet(ctx, slug)
	if err != nil {
		return err
	}
	instance, err := g.repo.ComputeInstanceGet(ctx, team.ID)
	if err != nil {
		return err
	}

	go g.deleteComputeInstance(ctx, instance.InstanceName)

	if err := g.repo.ComputeInstanceDelete(ctx, team.ID); err != nil {
		return err
	}

	return nil
}

func (g *Google) createComputeInstance(ctx context.Context, users []string, teamSlug, name, machineType string) {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return
	}

	exists, err := g.computeInstanceExists(ctx, name)
	if err != nil {
		g.log.WithError(err).Errorf("create compute instance %v", name)
		return
	}
	if exists {
		g.log.Infof("create compute instance: compute instance %v already exists", name)
		return
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"create",
		name,
		fmt.Sprintf("--zone=%v", computeZone),
		fmt.Sprintf("--machine-type=%v", machineType),
		fmt.Sprintf("--network-interface=%v", g.vmNetworkConfig),
		fmt.Sprintf("--labels=created-by=knorten,team=%v", teamSlug),
		"--metadata=block-project-ssh-keys=TRUE",
		"--no-service-account",
		"--no-scopes",
	)

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Errorf("create compute instance %v", name)
		return
	}

	for _, u := range users {
		if err := g.addOwnerBinding(ctx, name, u); err != nil {
			g.log.WithError(err).Errorf("create compute instance %v, add owner binding for user %v", name, u)
			return
		}
		if err := g.grantKnadaUserRole(ctx, u); err != nil {
			g.log.WithError(err).Errorf("create compute instance %v, grant knada-user project role for user %v", name, u)
			return
		}
	}
}

func (g *Google) computeInstanceExists(ctx context.Context, computeInstance string) (bool, error) {
	computeInstances, err := g.listComputeInstances(ctx)
	if err != nil {
		return false, err
	}

	for _, c := range computeInstances {
		if c.Name == computeInstance {
			return true, nil
		}
	}

	return false, nil
}

func (g *Google) listComputeInstances(ctx context.Context) ([]*computeInstance, error) {
	listCmd := exec.Command(
		"gcloud",
		"compute",
		"instances",
		"list",
		"--format=json",
		fmt.Sprintf("--project=%v", g.project))

	buf := &bytes.Buffer{}
	listCmd.Stdout = buf
	listCmd.Stderr = os.Stderr
	if err := listCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Error("list compute instances")
		return nil, err
	}

	computeInstances := []*computeInstance{}
	if err := json.Unmarshal(buf.Bytes(), &computeInstances); err != nil {
		return nil, err
	}

	return computeInstances, nil
}

func (g *Google) updateOwners(ctx context.Context, instance string, current, new []string) error {
	for _, nu := range new {
		if err := g.addOwnerBinding(ctx, instance, nu); err != nil {
			return err
		}
	}

	for _, m := range current {
		if !containsUser(new, m) {
			if err := g.removeOwnerBinding(ctx, instance, m); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Google) addOwnerBinding(ctx context.Context, instance, user string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	addCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"add-iam-policy-binding",
		instance,
		fmt.Sprintf("--role=%v", ownerRole),
		fmt.Sprintf("--member=user:%v", user),
		fmt.Sprintf("--zone=%v", computeZone),
	)

	buf := &bytes.Buffer{}
	addCmd.Stdout = buf
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Errorf("adding compute instance iam owner rolebinding for %v", user)
		return err
	}

	return nil
}

func (g *Google) grantKnadaUserRole(ctx context.Context, user string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"projects",
		"add-iam-policy-binding",
		g.project,
		fmt.Sprintf("--member=user:%v", user),
		fmt.Sprintf("--role=%v", knadaUserRole),
		"--condition=None")

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Errorf("create knada-user iam binding for user %v", user)
		return err
	}

	return nil
}

func (g *Google) removeOwnerBinding(ctx context.Context, instance, user string) error {
	addCmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"remove-iam-policy-binding",
		instance,
		fmt.Sprintf("--role=%v", ownerRole),
		fmt.Sprintf("--member=%v", user),
		fmt.Sprintf("--zone=%v", computeZone),
	)

	buf := &bytes.Buffer{}
	addCmd.Stdout = buf
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Errorf("removing compute instance iam owner rolebinding for %v", user)
		return err
	}

	return nil
}

func (g *Google) deleteComputeInstance(ctx context.Context, instance string) error {
	if g.dryRun {
		g.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	cmd := exec.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"delete",
		instance,
		fmt.Sprintf("--zone=%v", computeZone),
	)

	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		io.Copy(os.Stdout, buf)
		g.log.WithError(err).Errorf("delete compute instance %v", instance)
		return err
	}

	return nil
}

func containsUser(users []string, user string) bool {
	for _, u := range users {
		if "user:"+u == strings.ToLower(user) {
			return true
		}
	}

	return false
}

func TeamToComputeInstanceName(teamID string) string {
	return fmt.Sprintf("compute-%v", teamID)
}
