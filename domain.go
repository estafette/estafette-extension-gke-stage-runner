package main

import "fmt"

// Params are the parameters passed to this extension via the custom properties of the estafette stage
type Params struct {
	Credentials string            `json:"credentials,omitempty" yaml:"credentials,omitempty"`
	EnvVars     map[string]string `json:"remoteEnv,omitempty" yaml:"remoteEnv,omitempty"`
	RemoteImage string            `json:"remoteImage,omitempty" yaml:"remoteImage,omitempty"`
	Namespace   string            `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// SetDefaults fills in empty fields with convention-based defaults
func (p *Params) SetDefaults(releaseName string) {
	// default credentials to release name prefixed with gke if no override in stage params
	if p.Credentials == "" && releaseName != "" {
		p.Credentials = fmt.Sprintf("gke-%v", releaseName)
	}
}
