package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	ignitionapi "github.com/coreos/ignition/v2/config/v3_1/types"
	capiproviderv1alpha1 "github.com/eranco74/cluster-api-provider-agent/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	hiveext "github.com/openshift/assisted-service/api/hiveextension/v1beta1"
	aiv1beta1 "github.com/openshift/assisted-service/api/v1beta1"
	v1 "github.com/openshift/custom-resource-status/conditions/v1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sutilspointer "k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	_ = aiv1beta1.AddToScheme(scheme.Scheme)
	_ = hivev1.AddToScheme(scheme.Scheme)
	_ = hiveext.AddToScheme(scheme.Scheme)
	_ = capiproviderv1alpha1.AddToScheme(scheme.Scheme)
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func newAgentMachineRequest(agentMachine *capiproviderv1alpha1.AgentMachine) ctrl.Request {
	namespacedName := types.NamespacedName{
		Namespace: agentMachine.ObjectMeta.Namespace,
		Name:      agentMachine.ObjectMeta.Name,
	}
	return ctrl.Request{NamespacedName: namespacedName}
}

func newAgentMachine(name, namespace string, spec capiproviderv1alpha1.AgentMachineSpec, ctx context.Context, c client.Client, missingLink bool) *capiproviderv1alpha1.AgentMachine {
	clusterDeployment := hivev1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-deployment-%s", name),
			Namespace: namespace,
		},
		Spec: hivev1.ClusterDeploymentSpec{},
	}
	Expect(c.Create(ctx, &clusterDeployment)).To(BeNil())

	agentCluster := capiproviderv1alpha1.AgentCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("agent-cluster-%s", name),
			Namespace: namespace,
		},
		Spec:   capiproviderv1alpha1.AgentClusterSpec{ClusterName: "foo", BaseDomain: "example.com"},
		Status: capiproviderv1alpha1.AgentClusterStatus{ClusterDeploymentRef: capiproviderv1alpha1.ClusterDeploymentReference{Namespace: namespace, Name: clusterDeployment.Name}},
	}
	Expect(c.Create(ctx, &agentCluster)).To(BeNil())

	cluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-%s", name),
			Namespace: namespace,
		},
		Spec:   clusterv1.ClusterSpec{InfrastructureRef: &corev1.ObjectReference{Namespace: agentCluster.Namespace, Name: agentCluster.Name}},
		Status: clusterv1.ClusterStatus{},
	}
	Expect(c.Create(ctx, &cluster)).To(BeNil())

	machine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("machine-%s", name),
			Namespace: namespace,
			Labels:    make(map[string]string),
		},
		Spec:   clusterv1.MachineSpec{},
		Status: clusterv1.MachineStatus{},
	}
	machine.ObjectMeta.Labels[clusterv1.ClusterLabelName] = cluster.Name
	Expect(c.Create(ctx, &machine)).To(BeNil())

	machineOwnerRef := metav1.OwnerReference{APIVersion: "cluster.x-k8s.io/v1beta1", Kind: "Machine", Name: machine.Name}
	agentMachine := capiproviderv1alpha1.AgentMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   spec,
		Status: capiproviderv1alpha1.AgentMachineStatus{},
	}
	if !missingLink {
		agentMachine.ObjectMeta.OwnerReferences = append(agentMachine.ObjectMeta.OwnerReferences, machineOwnerRef)
	}
	return &agentMachine
}

func newAgent(name, namespace string, spec aiv1beta1.AgentSpec) *aiv1beta1.Agent {
	return &aiv1beta1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   spec,
		Status: aiv1beta1.AgentStatus{},
	}
}

var _ = Describe("agentmachine reconcile", func() {
	var (
		c             client.Client
		amr           *AgentMachineReconciler
		ctx           = context.Background()
		mockCtrl      *gomock.Controller
		testNamespace = "test-namespace"
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		mockCtrl = gomock.NewController(GinkgoT())

		amr = &AgentMachineReconciler{
			Client: c,
			Scheme: scheme.Scheme,
			Log:    logrus.New(),
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("agentMachine update status ready", func() {
		agent := newAgent("agent-1", testNamespace, aiv1beta1.AgentSpec{Approved: true,
			IgnitionEndpointToken: "foo",
			ClusterDeploymentName: &aiv1beta1.ClusterReference{Namespace: testNamespace, Name: "dep"}})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "True"})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.InstalledCondition, Status: "True"})
		Expect(c.Create(ctx, agent)).To(BeNil())

		agentMachine := newAgentMachine("agentMachine-1", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		agentMachine.Status.AgentRef = &capiproviderv1alpha1.AgentReference{Namespace: testNamespace, Name: "agent-1"}
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{}))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agentMachine-1"}, agentMachine)).To(BeNil())
		Expect(agentMachine.Status.Ready).To(BeEquivalentTo(true))
	})

	It("agentMachine no agents", func() {
		// Agent0: not approved
		agent0 := newAgent("agent-0", testNamespace, aiv1beta1.AgentSpec{Approved: false})
		agent0.Status.Conditions = append(agent0.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent0.Status.Conditions = append(agent0.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent0)).To(BeNil())

		agentMachine := newAgentMachine("agentMachine-0", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))
	})

	It("agentMachine set clusterref later", func() {
		agent := newAgent("agent-1", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent)).To(BeNil())

		// Create AgentMachine with missing label in Machine
		agentMachine := newAgentMachine("agentMachine-1", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, true)
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{RequeueAfter: defaultRequeueAfterOnError}))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agentMachine-1"}, agentMachine)).To(BeNil())
		Expect(agentMachine.Status.AgentRef.Name).To(BeEquivalentTo("agent-1"))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agent-1"}, agent)).To(BeNil())
		Expect(agent.Spec.ClusterDeploymentName).To(BeNil())

		// Create missing link and reconcile again
		machineOwnerRef := metav1.OwnerReference{APIVersion: "cluster.x-k8s.io/v1beta1", Kind: "Machine", Name: "machine-agentMachine-1"}
		agentMachine.ObjectMeta.OwnerReferences = append(agentMachine.ObjectMeta.OwnerReferences, machineOwnerRef)
		c.Update(ctx, agentMachine)

		result, err = amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agentMachine-1"}, agentMachine)).To(BeNil())
		Expect(agentMachine.Status.AgentRef.Name).To(BeEquivalentTo("agent-1"))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agent-1"}, agent)).To(BeNil())
		Expect(agent.Spec.ClusterDeploymentName.Name).To(BeEquivalentTo("cluster-deployment-agentMachine-1"))

	})

	It("agentMachine set ignition endpoint", func() {
		agent := newAgent("agent-1", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent.Status.Conditions = append(agent.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent)).To(BeNil())

		agentMachine := newAgentMachine("agentMachine-1", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		ignConfig := ignitionapi.Config{
			Ignition: ignitionapi.Ignition{
				Version: "3.1.0",
				Security: ignitionapi.Security{
					TLS: ignitionapi.TLS{
						CertificateAuthorities: []ignitionapi.Resource{
							{
								Source: k8sutilspointer.StringPtr("data:text/plain;base64,encodedCACert"),
							},
						},
					},
				},
				Config: ignitionapi.IgnitionConfig{
					Merge: []ignitionapi.Resource{
						{
							Source: k8sutilspointer.StringPtr("https://endpoint/ignition"),
							HTTPHeaders: []ignitionapi.HTTPHeader{
								{
									Name:  "Authorization",
									Value: k8sutilspointer.StringPtr("Bearer encodedToken"),
								},
							},
						},
					},
				},
			},
		}
		userDataValue, err := json.Marshal(ignConfig)
		Expect(err).To(BeNil())
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "userdata-secret",
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"value": userDataValue,
			},
		}
		Expect(c.Create(ctx, &secret)).To(BeNil())

		machine := &clusterv1.Machine{}
		machineRef := types.NamespacedName{Namespace: agentMachine.Namespace, Name: agentMachine.ObjectMeta.OwnerReferences[0].Name}
		Expect(c.Get(ctx, machineRef, machine)).To(BeNil())
		machine.Spec.Bootstrap.DataSecretName = &secret.ObjectMeta.Name
		c.Update(ctx, machine)

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agent-1"}, agent)).To(BeNil())
		Expect(agent.Spec.IgnitionEndpointToken).To(BeEquivalentTo("encodedToken"))
		Expect(agent.Spec.MachineConfigPool).To(BeEquivalentTo("ignition"))
	})

	It("agentMachine find agent end-to-end", func() {
		// Agent0: not approved
		agent0 := newAgent("agent-0", testNamespace, aiv1beta1.AgentSpec{Approved: false})
		agent0.Status.Conditions = append(agent0.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent0.Status.Conditions = append(agent0.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent0)).To(BeNil())
		// Agent1: already bound
		agent1 := newAgent("agent-1", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent1.Status.Conditions = append(agent1.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "True"})
		agent1.Status.Conditions = append(agent1.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent1)).To(BeNil())
		// Agent2: OK to use, but another AgentMachine is already taking it
		agent2 := newAgent("agent-2", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent2.Status.Conditions = append(agent2.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent2.Status.Conditions = append(agent2.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent2)).To(BeNil())
		agentMachine2 := newAgentMachine("agentMachine-2", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		agentMachine2.Status.AgentRef = &capiproviderv1alpha1.AgentReference{Namespace: testNamespace, Name: "agent-2"}
		Expect(c.Create(ctx, agentMachine2)).To(BeNil())
		// Agent3: validations are failing
		agent3 := newAgent("agent-3", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent3.Status.Conditions = append(agent3.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent3.Status.Conditions = append(agent3.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "False"})
		Expect(c.Create(ctx, agent3)).To(BeNil())
		// Agent4: should use this one
		agent4 := newAgent("agent-4", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent4.Status.Conditions = append(agent4.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent4.Status.Conditions = append(agent4.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent4)).To(BeNil())
		// Agent5: Also in use by another AgentMachine
		agent5 := newAgent("agent-5", testNamespace, aiv1beta1.AgentSpec{Approved: true})
		agent5.Status.Conditions = append(agent5.Status.Conditions, v1.Condition{Type: aiv1beta1.BoundCondition, Status: "False"})
		agent5.Status.Conditions = append(agent5.Status.Conditions, v1.Condition{Type: aiv1beta1.ValidatedCondition, Status: "True"})
		Expect(c.Create(ctx, agent5)).To(BeNil())
		agentMachine5 := newAgentMachine("agentMachine-5", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		agentMachine5.Status.AgentRef = &capiproviderv1alpha1.AgentReference{Namespace: testNamespace, Name: "agent-5"}
		Expect(c.Create(ctx, agentMachine5)).To(BeNil())

		agentMachine := newAgentMachine("agentMachine-4", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agentMachine-4"}, agentMachine)).To(BeNil())
		Expect(agentMachine.Status.AgentRef.Name).To(BeEquivalentTo("agent-4"))

		Expect(c.Get(ctx, types.NamespacedName{Namespace: testNamespace, Name: "agent-4"}, agent4)).To(BeNil())
		Expect(agent4.Spec.ClusterDeploymentName.Name).To(BeEquivalentTo("cluster-deployment-agentMachine-4"))
	})

	It("non-existing agentMachine", func() {
		agentMachine := newAgentMachine("agentMachine-1", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		nonExistingAgentMachine := newAgentMachine("agentMachine-2", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(nonExistingAgentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{}))
	})

	It("agentMachine ready status", func() {
		agentMachine := newAgentMachine("agentMachine-1", testNamespace, capiproviderv1alpha1.AgentMachineSpec{}, ctx, c, false)
		agentMachine.Status.Ready = true
		Expect(c.Create(ctx, agentMachine)).To(BeNil())

		result, err := amr.Reconcile(ctx, newAgentMachineRequest(agentMachine))
		Expect(err).To(BeNil())
		Expect(result).To(Equal(ctrl.Result{}))
	})
})
