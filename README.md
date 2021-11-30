# cluster-api-provider-agent
Kubernetes-native declarative infrastructure for agent-based installation.

cluster-api-provider-agent serves as infrastructure provider for [Kubernetes cluster-api](https://github.com/kubernetes-sigs/cluster-api)

## Deploying the cluster-api-provider-agent provider
In order for your clusterctl to be aware of this provider, you need to update the clusterctl config located at "$HOME/.cluster-api/clusterctl.yaml" with the following field:\
providers:\
  &nbsp; \- name: "agent" \
    &nbsp;&nbsp;&nbsp;&nbsp;url: "https://github.com/eranco74/cluster-api-provider-agent/releases/latest/infrastructure-components.yaml" \
    &nbsp;&nbsp;&nbsp;&nbsp;type: "InfrastructureProvider"\
After adding that field to the config, you should run "clusterctl init --infrastructure agent" to set up the provider.

## How to install cluster-api-provider-agent

cluster-api-provider-agent is deployed into an existing OpenShift / Kubernetes cluster.

**Prerequisites:**
* Admin access to the OpenShift / Kubernetes cluster specified by the KUBECONFIG environment variable

Build the cluster-api-provider-agent image and push it to a container image repository:
```shell
make docker-build docker-push IMG=<your docker repository>:`git log -1 --short`
```

Deploy cluster-api-provider-agent to your cluster:
```shell
make deploy IMG=<your docker repository>:`git log -1 --short`
```

## Design
cluster-api-provider-agent utilizes the [Infrastructure Operator](https://github.com/openshift/assisted-service) for adjusting the amount of workers in an OpenShift cluster. The CRDs that it manages are:
 * AgentCluster
 * AgentMachine
 * AgentMachineTemplate

## High-level flow
 1. Using the Infrastructure Operator, create an InfraEnv suitable for your environment.
 1. Download the Discovery ISO from the InfraEnv's download URL and use it to boot one or more hosts. Each booted host will automatically run an agent process which will create an Agent CR in the InfraEnv's namespace.
 1. Approve Agents that you recognize and set any necessary properties (e.g., hostname, installation disk).
 1. When a new Machine is created, the CAPI provider will find an available Agent to associate with the Machine, and trigger its installation via the Infrastructure Operator.

## Status
Until now this CAPI provider has been tested for resizing [HyperShift](https://github.com/openshift/hypershift) Hosted Clusters. It also has the following limitations:
 * The [cluster machine approver](https://github.com/openshift/cluster-machine-approver) currently won't approve the *kubelet-serving* CSRs until you take the ProviderID from the appropriate Machine and add it to the Node's Spec.
 * The reprovisioning flow currently requires manually rebooting the host with the Discovery ISO.
 * The CAPI provider does not yet have cluster lifecycle features - it adds and removes nodes from an existing cluster.
 * The CAPI provider currently selects the first free Agent that is approved and whose validations are passing. It will be smarter in the future.
