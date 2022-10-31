package helm

type JupyterValues struct {
	JupyterGlobalValues
	JupyterTeamValues
}

type JupyterGlobalValues struct {
	ImageName string
	ImageTag  string
}

type JupyterTeamValues struct {
	ProxyToken string
}
