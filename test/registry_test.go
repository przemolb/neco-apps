package test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func prepareRegistry() {
	It("should prepare resources", func() {
		By("creating pods")
		podsYaml := `
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
`
		_, stderr, err := ExecAtWithInput(boot0, []byte(podsYaml), "kubectl", "apply", "-f", "-")
		Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr)
	})

}

func testRegistry() {
	It("should cache containers on mirror registries", func() {
		stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "registry", "get", "service", "registry-elastic",
			"--output=jsonpath={.spec.clusterIP}")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		elasticAddr := string(stdout)

		stdout, stderr, err = ExecAt(boot0, "kubectl", "-n", "registry", "get", "service", "registry-ghcr",
			"--output=jsonpath={.spec.clusterIP}")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		ghcrAddr := string(stdout)

		stdout, stderr, err = ExecAt(boot0, "kubectl", "-n", "registry", "get", "service", "registry-quay",
			"--output=jsonpath={.spec.clusterIP}")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		quayAddr := string(stdout)

		type catalog struct {
			Repositories []string `json:"repositories"`
		}

		By("checking docker.elastic.co")
		stdout, stderr, err = ExecAt(boot0, "curl", fmt.Sprintf("http://%s:5000/v2/_catalog", elasticAddr))
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		elasticCatalog := catalog{}
		err = json.Unmarshal(stdout, &elasticCatalog)
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		Expect(elasticCatalog.Repositories).Should(ContainElement("elasticsearch/elasticsearch-oss"))

		By("checking ghcr.io")
		stdout, stderr, err = ExecAt(boot0, "curl", fmt.Sprintf("http://%s:5000/v2/_catalog", ghcrAddr))
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		ghcrCatalog := catalog{}
		err = json.Unmarshal(stdout, &ghcrCatalog)
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		Expect(ghcrCatalog.Repositories).Should(ContainElement("cybozu-go/moco"))

		By("checking quay.io")
		stdout, stderr, err = ExecAt(boot0, "curl", fmt.Sprintf("http://%s:5000/v2/_catalog", quayAddr))
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		quayCatalog := catalog{}
		err = json.Unmarshal(stdout, &quayCatalog)
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
		Expect(quayCatalog.Repositories).Should(ContainElement("cybozu/ubuntu-debug"))
	})
}
