package cnpg_test

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/k8s/cnpg"
	"github.com/sebdah/goldie/v2"
	"sigs.k8s.io/yaml"
)

func TestNew(t *testing.T) {
	testCases := []struct {
		name    string
		desc    string
		cluster *cnpgv1.Cluster
	}{
		{
			name: "default-cluster",
			desc: "Create a new default cluster",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
			),
		},
		{
			name: "cluster-with-backup",
			desc: "Create a new cluster with backup",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
				cnpg.WithBackup("7d"),
			),
		},
		{
			name: "cluster-with-storage-size",
			desc: "Create a new cluster with storage size",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
				cnpg.WithStorageSize("2Gi"),
			),
		},
		{
			name: "cluster-with-instance-count",
			desc: "Create a new cluster with instance count",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
				cnpg.WithInstanceCount(3),
			),
		},
		{
			name: "cluster-with-requests",
			desc: "Create a new cluster with requests",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
				cnpg.WithRequests("1000m", "1Gi"),
			),
		},
		{
			name: "cluster-with-app-label",
			desc: "Create a new cluster with app label",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
				cnpg.WithAppLabel("test-app"),
			),
		},
		{
			name: "cluster-with-monitoring",
			desc: "Create a new cluster with monitoring",
			cluster: cnpg.NewCluster(
				"test-cluster",
				"test-namespace",
				"test-database",
				"test-owner",
				cnpg.WithMonitoring(true),
			),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := goldie.New(t)

			output, err := yaml.Marshal(tc.cluster)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, output)
		})
	}
}

func TestNewScheduledBackup(t *testing.T) {
	testCases := []struct {
		name   string
		desc   string
		backup *cnpgv1.ScheduledBackup
	}{
		{
			name: "default-scheduled-backup",
			desc: "Create a new default scheduled backup",
			backup: cnpg.NewScheduledBackup(
				"test-scheduled-backup",
				"test-namespace",
				"test-cluster",
			),
		},
		{
			name: "scheduled-backup-with-schedule",
			desc: "Create a new scheduled backup with a different schedule",
			backup: cnpg.NewScheduledBackup(
				"test-scheduled-backup",
				"test-namespace",
				"test-cluster",
				cnpg.WithSchedule("0 0 0 5 0 0"),
			),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := goldie.New(t)

			output, err := yaml.Marshal(tc.backup)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, output)
		})
	}
}
