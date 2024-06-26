package core

import (
	"fmt"

	"github.com/navikt/knorten/pkg/k8s/meta"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	serviceAccountKind = "ServiceAccount"
	secretKind         = "Secret"
	namespaceKind      = "Namespace"
	apiVersion         = "v1"
)

func NewSecret(name, namespace string, data map[string]string) *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       secretKind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
		StringData: data,
	}
}

type NamespaceOption func(*v1.Namespace)

func WithTeamNamespaceLabel() NamespaceOption {
	return func(ns *v1.Namespace) {
		ns.Labels[meta.TeamNamespaceLabel] = "true"
	}
}

func NewNamespace(name string, options ...NamespaceOption) *v1.Namespace {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       namespaceKind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: meta.DefaultLabels(),
		},
	}

	for _, option := range options {
		option(ns)
	}

	return ns
}

type ServiceAccountOption func(*v1.ServiceAccount)

func WithGKEIAMAccountAnnotation(teamID, gcpProject string) ServiceAccountOption {
	return func(sa *v1.ServiceAccount) {
		sa.Annotations = map[string]string{
			"iam.gke.io/gcp-service-account": fmt.Sprintf("%v@%v.iam.gserviceaccount.com", teamID, gcpProject),
		}
	}
}

func NewServiceAccount(name, namespace string, options ...ServiceAccountOption) *v1.ServiceAccount {
	sa := &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       serviceAccountKind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
	}

	for _, option := range options {
		option(sa)
	}

	return sa
}
