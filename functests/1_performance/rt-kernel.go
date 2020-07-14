package __performance

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	testutils "github.com/openshift-kni/performance-addon-operators/functests/utils"
	testclient "github.com/openshift-kni/performance-addon-operators/functests/utils/client"
	"github.com/openshift-kni/performance-addon-operators/functests/utils/discovery"
	"github.com/openshift-kni/performance-addon-operators/functests/utils/nodes"
	"github.com/openshift-kni/performance-addon-operators/functests/utils/pods"
	"github.com/openshift-kni/performance-addon-operators/functests/utils/profiles"
	performancev1alpha1 "github.com/openshift-kni/performance-addon-operators/pkg/apis/performance/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("[performance]RT Kernel", func() {
	var testpod *corev1.Pod
	var profile *performancev1alpha1.PerformanceProfile

	testutils.BeforeAll(func() {
		key := types.NamespacedName{
			Name: testutils.PerformanceProfileName,
		}
		err := testclient.Client.Get(context.TODO(), key, profile)
		Expect(err).ToNot(HaveOccurred(), "failed to get the performance profile")

		profilesCount, err := profiles.GetProfilesCount()
		Expect(err).ToNot(HaveOccurred())
		excludedProfiles := []string{}

		for profilesCount > 0 {
			if profile.Spec.RealTimeKernel != nil && *profile.Spec.RealTimeKernel.Enabled == true {
				break
			}
			excludedProfiles = append(excludedProfiles, profile.Name)
			profile, err = discovery.GetDiscoveryPerformanceProfile(excludedProfiles...)
			profilesCount--
		}
		if profilesCount == 0 {
			Skip("Skipping RT Kernel tests since no profile found with RT kernel set")
		}
	})

	AfterEach(func() {
		if testpod == nil {
			return
		}
		if err := testclient.Client.Delete(context.TODO(), testpod); err == nil {
			pods.WaitForDeletion(testpod, 60*time.Second)
		}
	})

	It("[test_id:26861][crit:high][vendor:cnf-qe@redhat.com][level:acceptance] should have RT kernel enabled", func() {

		Eventually(func() string {

			// run uname -a in a busybox pod and get logs
			testpod = pods.GetTestPod()
			testpod.Namespace = testutils.NamespaceTesting
			testpod.Spec.Containers[0].Command = []string{"uname", "-a"}
			testpod.Spec.RestartPolicy = corev1.RestartPolicyNever
			testpod.Spec.NodeSelector = testutils.NodeSelectorLabels

			if err := testclient.Client.Create(context.TODO(), testpod); err != nil {
				return ""
			}

			if err := pods.WaitForPhase(testpod, corev1.PodSucceeded, 60*time.Second); err != nil {
				return ""
			}

			logs, err := pods.GetLogs(testclient.K8sClient, testpod)
			if err != nil {
				return ""
			}

			return logs

		}, 15*time.Minute, 30*time.Second).Should(ContainSubstring("PREEMPT RT"))

	})

	It("[test_id:28526][crit:high][vendor:cnf-qe@redhat.com][level:acceptance] a node without performance profile applied should not have RT kernel installed", func() {

		By("Skipping test if cluster does not have another available worker node")
		nonPerformancesWorkers, err := nodes.GetNonPerformancesWorkers()
		Expect(err).ToNot(HaveOccurred())

		if len(nonPerformancesWorkers) == 0 {
			Skip("Skipping test because there are no additional non-cnf worker nodes")
		}

		cmd := []string{"uname", "-a"}
		kernel, err := nodes.ExecCommandOnNode(cmd, &nonPerformancesWorkers[0])
		Expect(err).ToNot(HaveOccurred(), "failed to execute uname")
		Expect(kernel).To(ContainSubstring("Linux"), "Node should have Linux string")
		Expect(kernel).NotTo(ContainSubstring("PREEMPT RT"), "Node should have non-RT kernel")
	})

})
