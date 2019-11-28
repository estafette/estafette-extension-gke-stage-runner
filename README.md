# extensions/gke-stage-runner

This extension runs a stage remotely in a GKE cluster

## Parameters

| Parameter             | Type              | Values                                                                                                                  |
| --------------------- | ----------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `credentials`         | string            | To set a specific set of type `kubernetes-engine` credentials; defaults to the release target name prefixed with `gke-` |
| `namespace`           | string            | The namespace in which to run the container remotely; defaults to the namespace defined in the credentials              |
| `remoteEnv`           | map[string]string | Environment variables to pass to the remote container                                                                   |
| `remoteImage`         | string            | The full docker image path including repository and tag to run remotely                                                 |

## Usage

In order to use this extension in your `.estafette.yaml` manifest use the following snippet:

```yaml
  run-remotely:
    image: extensions/gke-stage-runner:stable
    remoteImage: alpine:3.10
    env:
      MYENVVAR: value
    namespace: staging
```
