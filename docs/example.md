# cluster-api-provider-agent - Getting Started Guide

This document is a step-by-step guide that demonstrates how to add a worker to HyperShift cluster end to end.

## What To Expect
* A walk through of custom resource definitions (CRDs) and understand how they relate to each other.

* A clear understanding of what happened, both in OpenShift / Kubernetes and assisted-service backend per each action described below.

## How To Use This guide

* Make sure that for each step you follow, you understand how and why it is done.

* You are encouraged to copy and paste the below-mentioned commands to your terminal and see things in action.

* Note the inline comments in the `yaml` examples. There is no need to remove those in case you which to copy and create the resources directly in your environment.

## Assumptions
* You have deployed both the operator and assisted-service by, for example, [using these instructions](https://github.com/openshift/assisted-service/blob/master/docs/operator.md).
* **Boot it yourself**: You are able to use the generated discovery image to boot your host.
* You have an running HyperShift cluster on 


## Cluster Prerequisites

### 1. Create a namespace for your resources

```yaml
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Namespace
metadata:
  name: demo-worker4 # Use any name
EOF
```

## Add agents (Boot it youreslf)
[Generate Cluster Discovery Image](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/kube-api-getting-started.md#generate-cluster-discovery-image)
[Dowload the image](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/kube-api-getting-started.md#1-download-the-image)
[Boot The Host From The Discovery Image](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/kube-api-getting-started.md#boot-the-host-from-the-discovery-image)
[Host discovery and approval](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/kube-api-getting-started.md#3-host-discovery-and-approval)

* At the end of the process you should see an approved agent
```bash
kubectl -n demo-worker4 get agents
```
```bash
NAME                                   CLUSTER        APPROVED
07e80ea9-200c-4f82-aff4-4932acb773d4   test-cluster   true
```

## Creating Cluster Resources

### Define an AgentCluster

#### 2. Create AgentCluster
* The `AgentCluster` resource describes everything about the cluster that is needed in order to add nodes to it.
```yaml
cat <<EOF | kubectl create -f -
apiVersion: capi-provider.agent-install.openshift.io/v1alpha1
kind: AgentCluster
metadata:
 name: myagentcluster
 namespace: clusters-demo  # use the namespace of the relevant hypershift cluster
 labels:
  cluster.x-k8s.io/v1alpha3: v1alpha1
  cluster.x-k8s.io/v1alpha4: v1alpha1
  cluster.x-k8s.io/v1beta1: v1alpha1
spec:  # all values below should match the values in the hypershift hostedcontrolplane
 releaseImage: quay.io/openshift-release-dev/ocp-release:4.8.12-x86_64
 clusterName: test-infra-cluster-assisted-installer
 baseDomain: redhat.com
 pullSecretRef:
  name: pull-secret
EOF
```

The cluster-api-provider-agent will create the following resources
1. [ClusterDepoyment](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/README.md#clusterdeployment)
2. [AgentClusterInstal](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/README.md#agentclusterinstall)
3. [ClusterImageSet](https://github.com/openshift/assisted-service/blob/master/docs/hive-integration/kube-api-select-ocp-versions.md#clusterimageset)



Lastly, when installed successfully:
```json
{
  "cluster_id": "02a89bb9-6141-4d14-a82e-42f254217502",
  "event_time": "2021-06-29T16:45:21.643Z",
  "message": "Successfully finished installing cluster test-cluster",
  "severity": "info"
}
```