# Makefile to update manifests

HELM_VERSION = 3.5.2

.PHONY: update-argocd
update-argocd:
	$(call get-latest-tag,argocd)
	curl -sLf -o argocd/base/upstream/install.yaml \
		https://raw.githubusercontent.com/argoproj/argo-cd/$(call upstream-tag,$(latest_tag))/manifests/install.yaml
	sed -i -E '/name:.*argocd$$/!b;n;s/newTag:.*$$/newTag: $(latest_tag)/' argocd/base/kustomization.yaml
	$(call get-latest-tag,dex)
	sed -i -E '/name:.*dex$$/!b;n;s/newTag:.*$$/newTag: $(latest_tag)/' argocd/base/kustomization.yaml
	$(call get-latest-tag,redis)
	sed -i -E '/name:.*redis$$/!b;n;s/newTag:.*$$/newTag: $(latest_tag)/' argocd/base/kustomization.yaml

.PHONY: update-kube-metrics-adapter
update-kube-metrics-adapter:
	$(call get-latest-tag,kube-metrics-adapter)
	rm -rf /tmp/kube-metrics-adapter
	cd /tmp; git clone -b $(call upstream-tag,$(latest_tag)) --depth 1 https://github.com/zalando-incubator/kube-metrics-adapter
	helm template \
		--set namespace=kube-metrics-adapter \
		--set enableExternalMetricsApi=true \
		--set service.internalPort=6443 \
		--set replicas=2 \
		/tmp/kube-metrics-adapter/docs/helm > kube-metrics-adapter/base/upstream/manifest.yaml
	rm -rf /tmp/kube-metrics-adapter
	sed -i 's/newTag: .*/newTag: $(latest_tag)/' kube-metrics-adapter/base/kustomization.yaml

.PHONY: update-prometheus-adapter
update-prometheus-adapter:
	$(call get-latest-tag,prometheus-adapter)
	sed -i -E \
		-e 's/^(          tag:).*$$/\1 $(latest_tag)/' \
		-e 's/^(    targetRevision:).*$$/\1 $(CHART_VERSION)/' \
		argocd-config/base/prometheus-adapter.yaml
	rm -rf /tmp/prometheus-adapter

# usage: get-latest-tag NAME
define get-latest-tag
$(eval latest_tag := $(shell curl -sf https://quay.io/api/v1/repository/cybozu/$1/tag/ | jq -r '.tags[] | .name' | awk '/.*\..*\..*\./ {print $$1; exit}'))
endef

# usage: upstream-tag 1.2.3.4
define upstream-tag
$(shell echo $1 | sed -E 's/^(.*)\.[[:digit:]]+$$/v\1/')
endef

.PHONY: setup
setup:
	curl -o /tmp/helm.tgz -fsL https://get.helm.sh/helm-v$(HELM_VERSION)-linux-amd64.tar.gz
	mkdir -p $$(go env GOPATH)/bin
	tar --strip-components=1 -C $$(go env GOPATH)/bin -xzf /tmp/helm.tgz linux-amd64/helm
	rm -f /tmp/helm.tgz
