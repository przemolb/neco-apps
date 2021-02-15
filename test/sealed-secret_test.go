package test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func prepareSealedSecret() {
	It("should create a Secret to be converted for SealedSecret", func() {
		By("creating a SealedSecret")
		secret := []byte(`
apiVersion: v1
kind: Secret
metadata:
  name: sealed-secret-test
  namespace: default
type: Opaque
data:
  foo: YmFy
`)
		stdout, stderr, err := ExecAtWithInput(boot0, secret, "kubeseal | kubectl apply -f -")
		Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
	})
}

func testSealedSecret() {
	It("should be working", func() {
		Eventually(func() error {
			_, stderr, err := ExecAt(boot0, "kubectl", "get", "secrets", "sealed-secret-test")
			if err != nil {
				return fmt.Errorf("failed to get secret: %s: %w", string(stderr), err)
			}
			return nil
		}).Should(Succeed())
	})
}
