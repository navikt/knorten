package cnpg

import (
	"fmt"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/k8s/meta"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	postgresVersion                = "16"
	defaultInstanceCount           = 2
	defaultVolumeSnapshotClassName = "cnpg-vsp"
	defaultStorageSize             = "10Gi"
	defaultRequestMemory           = "500Mi"
	defaultRequestCPU              = "100m"
	DefaultBackupRetentionPolicy   = "30d"
)

type ClusterOption func(*cnpgv1.Cluster)

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

func WithAppLabel(app string) ClusterOption {
	return func(c *cnpgv1.Cluster) {
		if c.Labels == nil {
			c.Labels = make(map[string]string)
		}

		c.Labels[meta.AppLabel] = app
	}
}

func WithMonitoring(monitoring bool) ClusterOption {
	return func(c *cnpgv1.Cluster) {
		c.Spec.Monitoring = &cnpgv1.MonitoringConfiguration{
			EnablePodMonitor: monitoring,
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
			ImageName:             fmt.Sprintf("ghcr.io/cloudnative-pg/postgresql:%s", postgresVersion),
			PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
			PrimaryUpdateMethod:   cnpgv1.PrimaryUpdateMethodSwitchover,
			StorageConfiguration: cnpgv1.StorageConfiguration{
				Size: defaultStorageSize,
			},
			Affinity: cnpgv1.AffinityConfiguration{
				NodeSelector: map[string]string{"knada-infrastructure": ""},
				Tolerations: []v1.Toleration{
					{
						Key:      "knada-infrastructure",
						Operator: v1.TolerationOpExists,
						Effect:   v1.TaintEffectNoSchedule,
					},
				},
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

const (
	defaultScheduleEverydayAtMidnight   = "0 0 8 * * 1-5"
	scheduledBackupOwnerReferenceSelf   = "self"
	scheduledBackupMethodVolumeSnapshot = "volumeSnapshot"
	scheduledBackupKind                 = "ScheduledBackup"
)

type ScheduledBackupOption func(*cnpgv1.ScheduledBackup)

func WithSchedule(schedule string) ScheduledBackupOption {
	return func(sb *cnpgv1.ScheduledBackup) {
		sb.Spec.Schedule = schedule
	}
}

func NewScheduledBackup(name, namespace, clusterName string, options ...ScheduledBackupOption) *cnpgv1.ScheduledBackup {
	sb := &cnpgv1.ScheduledBackup{
		TypeMeta: metav1.TypeMeta{
			Kind:       scheduledBackupKind,
			APIVersion: cnpgv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
		Spec: cnpgv1.ScheduledBackupSpec{
			Immediate: boolPtr(true),
			Cluster: cnpgv1.LocalObjectReference{
				Name: clusterName,
			},
			BackupOwnerReference: scheduledBackupOwnerReferenceSelf,
			Method:               scheduledBackupMethodVolumeSnapshot,
			Schedule:             defaultScheduleEverydayAtMidnight,
		},
	}

	for _, option := range options {
		option(sb)
	}

	return sb
}

func boolPtr(b bool) *bool {
	return &b
}
