package test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

func prepareMoco() {
	It("should deploy mysqlcluster", func() {
		// This manifest is based on the this example (https://github.com/cybozu-go/moco/blob/v0.7.0/docs/example_mysql_cluster.md).
		// Changed as follows.
		// - Change the namespace of all resources.
		// - Change the tag of quay.io/cybozu/moco-mysql image.
		// - Remove the `.spec.serviceTemplate` field from MySQLCluster resource.
		manifest := `apiVersion: moco.cybozu.com/v1alpha1
kind: MySQLCluster
metadata:
  name: my-cluster
  namespace: test-moco
spec:
  replicas: 3
  podTemplate:
    spec:
      containers:
      - name: mysqld
        image: quay.io/cybozu/moco-mysql:8.0.18
        resources:
          requests:
            memory: "1Gi"
        livenessProbe:
          exec:
            command: ["/moco-bin/moco-agent", "ping"]
          initialDelaySeconds: 5
          periodSeconds: 5
        readinessProbe:
          httpGet:
            path: /health
            port: 9080
          initialDelaySeconds: 10
          periodSeconds: 5
      - name: err-log
        image: quay.io/cybozu/filebeat:7.9.2.1
        args: ["-c", "/etc/filebeat.yml"]
        volumeMounts:
        - name: err-filebeat-config
          mountPath: /etc/filebeat.yml
          readOnly: true
          subPath: filebeat.yml
        - name: err-filebeat-data
          mountPath: /var/lib/filebeat
        - name: var-log
          mountPath: /var/log/mysql
          readOnly: true
        - name: tmp
          mountPath: /tmp
      - name: slow-log
        image: quay.io/cybozu/filebeat:7.9.2.1
        args: ["-c", "/etc/filebeat.yml"]
        volumeMounts:
        - name: slow-filebeat-config
          mountPath: /etc/filebeat.yml
          readOnly: true
          subPath: filebeat.yml
        - name: slow-filebeat-data
          mountPath: /var/lib/filebeat
        - name: var-log
          mountPath: /var/log/mysql
          readOnly: true
        - name: tmp
          mountPath: /tmp
      securityContext:
        runAsUser: 10000
        runAsGroup: 10000
        fsGroup: 10000
      volumes:
      - name: err-filebeat-config
        configMap:
          name: err-filebeat-config
      - name: err-filebeat-data
        emptyDir: {}
      - name: slow-filebeat-config
        configMap:
          name: slow-filebeat-config
      - name: slow-filebeat-data
        emptyDir: {}
  dataVolumeClaimTemplateSpec:
    storageClassName: topolvm-provisioner
    accessModes: [ "ReadWriteOnce" ]
    resources:
      requests:
        storage: 3Gi
  mysqlConfigMapName: my-cluster-mycnf
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cluster-mycnf
  namespace: test-moco
data:
  max_connections: "5000"
  max_connect_errors: "10"
  max_allowed_packet: 1G
  max_heap_table_size: 64M
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: err-filebeat-config
  namespace: test-moco
data:
  filebeat.yml: |-
    path.data: /var/lib/filebeat
    filebeat.inputs:
    - type: log
      enabled: true
      paths:
        - /var/log/mysql/mysql.err*
    output.console:
      codec.format:
        string: '%{[message]}'
    logging.files:
      path: /tmp
      name: filebeat
      keepfiles: 7
      permissions: 0644
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: slow-filebeat-config
  namespace: test-moco
data:
  filebeat.yml: |-
    path.data: /var/lib/filebeat
    filebeat.inputs:
    - type: log
      enabled: true
      paths:
        - /var/log/mysql/mysql.slow*
    output.console:
      codec.format:
        string: '%{[message]}'
    logging.files:
      path: /tmp
      name: filebeat
      keepfiles: 7
      permissions: 0644`

		By("creating mysqlcluster")
		createNamespaceIfNotExists("test-moco")
		_, stderr, err := ExecAtWithInput(boot0, []byte(manifest), "kubectl", "apply", "-f", "-")
		Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr)
	})
}

func testMoco() {
	It("should be deployed successfully", func() {
		Eventually(func() error {
			stdout, _, err := ExecAt(boot0, "kubectl", "--namespace=moco-system",
				"get", "deployment/moco-controller-manager", "-o=json")
			if err != nil {
				return err
			}
			deployment := new(appsv1.Deployment)
			err = json.Unmarshal(stdout, deployment)
			if err != nil {
				return err
			}

			if int(deployment.Status.AvailableReplicas) != 1 {
				return fmt.Errorf("AvailableReplicas is not 1: %d", int(deployment.Status.AvailableReplicas))
			}
			return nil
		}).Should(Succeed())
	})

	It("should work", func() {
		By("waiting mysqlcluster is ready")
		Eventually(func() error {
			stdout, _, err := ExecAt(boot0, "kubectl", "--namespace=test-moco", "get", "mysqlcluster/my-cluster", "-o", "jsonpath='{.status.ready}'")
			if err != nil {
				return err
			}

			if string(stdout) != "True" {
				return errors.New("MySQLCluster is not ready")
			}
			return nil
		}).Should(Succeed())

		By("running kubectl moco mysql")
		stdout, stderr, err := ExecAt(boot0, "kubectl", "moco", "-n", "test-moco", "mysql", "-u", "root", "my-cluster", "--", "--version")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		Expect(string(stdout)).Should(ContainSubstring("mysql  Ver 8"))
	})
}
