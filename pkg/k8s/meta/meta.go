package meta

const (
	Knorten        = "knorten.knada.io"
	ManagedByLabel = "managed-by"
)

func DefaultLabels() map[string]string {
	return map[string]string{
		ManagedByLabel: Knorten,
	}
}
