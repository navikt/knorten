package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/k8s/meta"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultImageName     = "ghcr.io/cloudnative-pg/postgresql:16.1"
	defaultInstanceCount = 2
	defaultVolumeSnapshotClassName = "cnpg-vps"
	defaultStorageSize             = "1Gi"
	defaultRequestMemory           = "512Mi"
	defaultRequestCPU              = "300m"
	DefaultBackupRetentionPolicy   = "30d"
)

type ClusterOption func(*cnpgv1.Cluster)

// WithBackup sets the backup configuration for the cluster,
// with the given retention policy.
func WithBackup(retentionPolicy string) ClusterOption {
	return func(c *cnpgv1.Cluster) {
		c.Spec.Backup = &cnpgv1.BackupConfiguration{
			VolumeSnapshot: &cnpgv1.VolumeSnapshotConfiguration{
				ClassName: defaultVolumeSnapshotClassName,
			},
			RetentionPolicy: retentionPolicy,
		}
	}
}

func WithStorageSize(size string) ClusterOption {
	return func(c *cnpgv1.Cluster) {
		c.Spec.StorageConfiguration.Size = size
	}
}

func WithInstanceCount(count int) ClusterOption {
	return func(c *cnpgv1.Cluster) {
		c.Spec.Instances = count
	}
}

func WithRequests(cpu, mem string) ClusterOption {
	return func(c *cnpgv1.Cluster) {
		c.Spec.Resources.Requests = v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(cpu),
			v1.ResourceMemory: resource.MustParse(mem),
		}
	}
}

func NewCluster(name, namespace, database, owner string, options ...ClusterOption) *cnpgv1.Cluster {
	c := &cnpgv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       cnpgv1.ClusterKind,
			APIVersion: cnpgv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
		Spec: cnpgv1.ClusterSpec{
			Instances:             defaultInstanceCount,
			PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
			StorageConfiguration: cnpgv1.StorageConfiguration{
				Size: defaultStorageSize,
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse(defaultRequestCPU),
					v1.ResourceMemory: resource.MustParse(defaultRequestMemory),
				},
			},
			Backup: &cnpgv1.BackupConfiguration{
				VolumeSnapshot: &cnpgv1.VolumeSnapshotConfiguration{
					ClassName: defaultVolumeSnapshotClassName,
				},
				RetentionPolicy: DefaultBackupRetentionPolicy,
			},
			Bootstrap: &cnpgv1.BootstrapConfiguration{
				InitDB: &cnpgv1.BootstrapInitDB{
					Database: database,
					Owner:    owner,
				},
			},
		},
	}

	for _, option := range options {
		option(c)
	}

	return c
}
