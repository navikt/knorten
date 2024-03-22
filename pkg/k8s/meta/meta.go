package meta

const (
	Knorten            = "knorten.knada.io"
	ManagedByLabel     = "managed-by"
	AppLabel           = "app"
	TeamNamespaceLabel = "team-namespace"
)

func DefaultLabels() map[string]string {
	return map[string]string{
		ManagedByLabel: Knorten,
	}
}
