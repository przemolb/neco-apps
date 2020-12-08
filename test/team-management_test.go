package test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func testTeamManagement() {
	It("should give authority of ephemeral containers to unprivileged team", func() {
		By("creating test pod")
		stdout, stderr, err := ExecAt(boot0, "kubectl", "run", "-n", "maneki", "neco-ephemeral-test", "--image=quay.io/cybozu/ubuntu-debug:18.04", "pause")
		Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s, err: %v", stdout, stderr, err)

		By("waiting the pod become ready")
		Eventually(func() error {
			stdout, _, err := ExecAt(boot0, "kubectl", "get", "-n", "maneki", "pod/neco-ephemeral-test", "-o=json")
			if err != nil {
				return err
			}
			po := new(corev1.Pod)
			err = json.Unmarshal(stdout, po)
			if err != nil {
				return fmt.Errorf("failed to get pod info: %w", err)
			}

			if po.Status.ContainerStatuses == nil || len(po.Status.ContainerStatuses) == 0 || !po.Status.ContainerStatuses[0].Ready {
				return fmt.Errorf("pod is not ready")
			}

			return nil
		}).Should(Succeed())

		By("adding a ephemeral container by unprivileged team")
		stdout, stderr, err = ExecAt(boot0, "kubectl", "alpha", "debug", "-i", "-n", "maneki", "neco-ephemeral-test", "--image=quay.io/cybozu/ubuntu-debug:18.04", "--target=neco-ephemeral-test", "--as=test", "--as-group=maneki", "--as-group=system:authenticated", "--", "echo a")
		Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
	})
}
