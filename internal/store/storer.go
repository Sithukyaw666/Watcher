package store

type DeploymentStore interface {
	AddDeployment(d Deployment) error
	GetLastSuccessfulDeployment() (*Deployment, error)
	GetAllDeployments() ([]Deployment, error)
	GetLastDeployment() (*Deployment, error)
}

type LastDeploymentQuerier interface {
	GetLastDeployment() (*Deployment, error)
}
type DeploymentQuerier interface {
	GetLastSuccessfulDeployment() (*Deployment, error)
	GetAllDeployments() ([]Deployment, error)
	GetLastDeployment() (*Deployment, error)
}
