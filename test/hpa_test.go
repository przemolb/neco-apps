package test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func prepareHPA() {
	const manifests = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hpa-resource
  namespace: sandbox
spec:
  selector:
    matchLabels:
      run: hpa-resource
  template:
    metadata:
      labels:
        run: hpa-resource
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 10000
        runAsGroup: 10000
      containers:
      - name: ubuntu
        image: quay.io/cybozu/ubuntu:20.04
        command: ["/bin/sh", "-c", "while true; do true; done"]
        resources:
          requests:
            cpu: 100m
          limits:
            cpu: 200m
---
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: hpa-resource
  namespace: sandbox
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hpa-resource
  minReplicas: 1
  maxReplicas: 2
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hpa-custom
  namespace: sandbox
spec:
  selector:
    matchLabels:
      run: hpa-custom
  template:
    metadata:
      labels:
        run: hpa-custom
    spec:
      containers:
      - name: testhttpd
        image: quay.io/cybozu/testhttpd:0
        resources:
          requests:
            cpu: 100m
---
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: hpa-custom
  namespace: sandbox
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hpa-custom
  minReplicas: 1
  maxReplicas: 2
  metrics:
  - type: Pods
    pods:
      metric:
        name: test_hpa_requests_per_second
      target:
        type: AverageValue
        averageValue: 10
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hpa-external
  namespace: sandbox
spec:
  selector:
    matchLabels:
      run: hpa-external
  template:
    metadata:
      labels:
        run: hpa-external
    spec:
      containers:
      - name: testhttpd
        image: quay.io/cybozu/testhttpd:0
        resources:
          requests:
            cpu: 100m
---
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: hpa-external
  namespace: sandbox
  annotations:
    metric-config.external.processed-events-per-second.prometheus/query: |
      scalar(test_hpa_external)
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hpa-external
  minReplicas: 1
  maxReplicas: 4
  metrics:
  - type: External
    external:
      metric:
        name: processed-events-per-second
        selector:
          matchLabels:
            type: prometheus
      target:
        type: AverageValue
        averageValue: 10
`

	It("should prepare resources for HPA tests", func() {
		_, stderr, err := ExecAtWithInput(boot0, []byte(manifests), "kubectl", "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred(), "stderr=%s", stderr)
	})
}

func testHPA() {
	It("should work for standard resources (CPU)", func() {
		Eventually(func() error {
			stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "sandbox", "get", "deployments", "hpa-resource", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get hpa-resource deployment: %s: %w", stderr, err)
			}
			dpl := &appsv1.Deployment{}
			if err := json.Unmarshal(stdout, dpl); err != nil {
				return err
			}
			if dpl.Spec.Replicas == nil || *dpl.Spec.Replicas != 2 {
				return errors.New("replicas of hpa-resource deployment is not 2")
			}
			return nil
		}).Should(Succeed())

		ExecSafeAt(boot0, "kubectl", "-n", "sandbox", "delete", "deployments", "hpa-resource")
	})

	It("should work for custom resources provided by prometheus-adapter", func() {
		By("waiting for the test Pod to be created")
		var pod *corev1.Pod
		Eventually(func() error {
			pods := &corev1.PodList{}
			stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "sandbox", "get", "pods", "-l", "run=hpa-custom", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get pod list: %s: %w", stderr, err)
			}
			if err := json.Unmarshal(stdout, pods); err != nil {
				return err
			}
			if len(pods.Items) != 1 {
				return errors.New("no hpa-custom pods")
			}
			pod = &pods.Items[0]
			return nil
		}).Should(Succeed())

		metric := fmt.Sprintf(`test_hpa_requests_per_second{namespace="sandbox",pod="%s"} 20`, pod.Name) + "\n"
		url := fmt.Sprintf("http://%s/metrics/job/some_job", bastionPushgatewayFQDN)

		By("checking the number of replicas increases")
		Eventually(func() error {
			_, stderr, err := ExecAtWithInput(boot0, []byte(metric), "curl", "-sf", "--data-binary", "@-", url)
			if err != nil {
				return fmt.Errorf("failed to push a metrics to pushgateway: %s: %w", stderr, err)
			}
			stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "sandbox", "get", "deployments", "hpa-custom", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get hpa-custom deployment: %s: %w", stderr, err)
			}
			dpl := &appsv1.Deployment{}
			if err := json.Unmarshal(stdout, dpl); err != nil {
				return err
			}
			if dpl.Spec.Replicas == nil || *dpl.Spec.Replicas != 2 {
				return errors.New("replicas of hpa-custom is not 2")
			}
			return nil
		}).Should(Succeed())

		ExecSafeAt(boot0, "kubectl", "-n", "sandbox", "delete", "deployments", "hpa-custom")
	})

	It("should work for external resources provided by kube-metrics-adapter", func() {
		metric := "test_hpa_external 23\n"
		url := fmt.Sprintf("http://%s/metrics/job/some_job", bastionPushgatewayFQDN)
		Eventually(func() error {
			_, stderr, err := ExecAtWithInput(boot0, []byte(metric), "curl", "-sf", "--data-binary", "@-", url)
			if err != nil {
				return fmt.Errorf("failed to push a metrics to pushgateway: %s: %w", stderr, err)
			}
			stdout, stderr, err := ExecAt(boot0, "kubectl", "-n", "sandbox", "get", "deployments", "hpa-external", "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get hpa-external deployment: %s: %w", stderr, err)
			}
			dpl := &appsv1.Deployment{}
			if err := json.Unmarshal(stdout, dpl); err != nil {
				return err
			}
			if dpl.Spec.Replicas == nil || *dpl.Spec.Replicas != 3 {
				return errors.New("replicas of hpa-external is not 3")
			}
			return nil
		}).Should(Succeed())

		ExecSafeAt(boot0, "kubectl", "-n", "sandbox", "delete", "deployments", "hpa-external")
	})
}
