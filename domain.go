package main

import "fmt"

// Params are the parameters passed to this extension via the custom properties of the estafette stage
type Params struct {
	Credentials string       `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	Namespace   string       `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Remote      RemoteParams `json:"remote,omitempty" yaml:"remote,omitempty"`
}

// RemoteParams has the parameters to run a remote stage
type RemoteParams struct {
	ContainerImage string            `json:"image,omitempty" yaml:"image,omitempty"`
	Shell          string            `json:"shell,omitempty" yaml:"shell,omitempty"`
	Commands       []string          `json:"commands,omitempty" yaml:"commands,omitempty"`
	EnvVars        map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// SetDefaults fills in empty fields with convention-based defaults
func (p *Params) SetDefaults(releaseName string) {
	// default credentials to release name prefixed with gke if no override in stage params
	if p.Credentials == "" && releaseName != "" {
		p.Credentials = fmt.Sprintf("gke-%v", releaseName)
	}

	if p.Remote.Shell == "" {
		p.Remote.Shell = "/bin/sh"
	}
}
