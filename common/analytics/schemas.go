package analytics

import "github.com/bentoml/yatai-schemas/modelschemas"

type ClusterInfoSchema struct {
	ClusterName     string `json:"cluster_name"`
	K8sClusterCount int    `json:"k8s_cluster_count"`
	K8sVersion      string `json:"k8s_version"`
	IsMinikube      bool   `json:"is_minikube"`
}

type CommonPropertiesSchema struct {
	YataiVersion  string            `json:"yatai_version"`
	ClusterInfo   ClusterInfoSchema `json:"cluster_info"`
	OrgsCount     int               `json:"orgs_count"`
	UsersCount    int               `json:"users_count"`
	ApiTokenCount int               `json:"api_token_count"`
}

type StoresUsageSchema struct {
	BentosCount int `json:"bentos_count"`
	ModelsCount int `json:"models_count"`
}

type ActiveDeploymentSchema struct {
	CreatedAt        string                               `json:"created_at"`
	LastUpdatedAt    string                               `json:"last_updated_at"`
	DeploymentConfig *modelschemas.DeploymentTargetConfig `json:"deployment_config"`
}

type DeploymentsUsageSchema struct {
	ActiveDeploymentsCount     int                        `json:"active_deployments_count"`
	TerminatedDeploymentsCount int                        `json:"terminated_deployments_count"`
	Deployments                *[]*ActiveDeploymentSchema `json:"deployments"`
}
