package test

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func prepareRegistry() {
	It("should add registry addr entry to /etc/hosts", func() {
		stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "ingress-forest", "get", "service", "envoy",
			"--output=jsonpath={.status.loadBalancer.ingress[0].ip}")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		addr := string(stdout)

		stdout, stderr, err = ExecAt(boot0, "kubectl", "get", "nodes", "--no-headers", "--output=custom-columns=:metadata.name")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		nodes := strings.Split(string(stdout), "\n")
		for _, node := range nodes {
			_, stderr, err = ExecAt(boot0, "ckecli", "ssh", node, fmt.Sprintf(`sudo tee -a /etc/hosts <<EOF
%s quay.registry.gcp0.dev-ne.co
%s ghcr.registry.gcp0.dev-ne.co
%s elastic.registry.gcp0.dev-ne.co
EOF
`, addr, addr, addr))
			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr)
		}
	})

	It("should prepare resources", func() {
		By("creating pods")
		deployYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: quay-ubuntu
  namespace: registry
spec:
  containers:
  - command:
    - /usr/local/bin/pause
    image: quay.io/cybozu/ubuntu-debug:18.04
    imagePullPolicy: Always
    name: ubuntu
  securityContext:
    runAsGroup: 1000
    runAsUser: 1000
---
apiVersion: v1
kind: Pod
metadata:
  name: ghcr-moco
  namespace: registry
spec:
  containers:
  - command:
    - /usr/local/bin/pause
    image: ghcr.io/cybozu-go/moco:0.3.1
    imagePullPolicy: Always
    name: moco
  securityContext:
    runAsGroup: 1000
    runAsUser: 1000
---
apiVersion: v1
kind: Pod
metadata:
  name: elastic-elasticsearch
  namespace: registry
spec:
  containers:
  - command:
    - /usr/local/bin/pause
    image: docker.elastic.co/elasticsearch/elasticsearch-oss:7.9.3
    imagePullPolicy: Always
    name: elasticsearch
  securityContext:
    runAsGroup: 1000
    runAsUser: 1000
---
`
		_, stderr, err := ExecAtWithInput(boot0, []byte(deployYAML), "kubectl", "apply", "-f", "-")
		Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr)
	})

}

func testRegistry() {
	It("should cache containers on mirror registries", func() {
		stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "ingress-forest", "get", "service", "envoy",
			"--output=jsonpath={.status.loadBalancer.ingress[0].ip}")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		addr := string(stdout)

		type catalog struct {
			Repositories []string `json:"repositories"`
		}

		By("checking quay.io")
		stdout, stderr, err = ExecAt(boot0, "curl",
			"-H", "'Host:quay.registry.gcp0.dev-ne.co'",
			"--resolve", fmt.Sprintf("'quay.registry.gcp0.dev-ne.co:80:%s'", addr),
			"http://quay.registry.gcp0.dev-ne.co/v2/_catalog")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		quayCatalog := catalog{}
		err = json.Unmarshal(stdout, &quayCatalog)
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		Expect(quayCatalog.Repositories).Should(ContainElement("cybozu/ubuntu-debug"))

		By("checking ghcr.io")
		stdout, stderr, err = ExecAt(boot0, "curl",
			"-H", "'Host:ghcr.registry.gcp0.dev-ne.co'",
			"--resolve", fmt.Sprintf("'ghcr.registry.gcp0.dev-ne.co:80:%s'", addr),
			"http://ghcr.registry.gcp0.dev-ne.co/v2/_catalog")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		ghcrCatalog := catalog{}
		err = json.Unmarshal(stdout, &ghcrCatalog)
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		Expect(ghcrCatalog.Repositories).Should(ContainElement("cybozu-go/moco"))

		By("checking docker.elastic.co")
		stdout, stderr, err = ExecAt(boot0, "curl",
			"-H", "'Host:elastic.registry.gcp0.dev-ne.co'",
			"--resolve", fmt.Sprintf("'elastic.registry.gcp0.dev-ne.co:80:%s'", addr),
			"http://elastic.registry.gcp0.dev-ne.co/v2/_catalog")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		elasticCatalog := catalog{}
		err = json.Unmarshal(stdout, &elasticCatalog)
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		Expect(elasticCatalog.Repositories).Should(ContainElement("elasticsearch/elasticsearch-oss"))
	})
}
