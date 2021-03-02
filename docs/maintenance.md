How to maintain neco-apps
=========================

- [argocd](#argocd)
- [cert-manager](#cert-manager)
- [customer-egress](#customer-egress)
- [elastic (ECK)](#elastic-eck)
- [external-dns](#external-dns)
- [kube-metrics-adapter](#kube-metrics-adapter)
- [ingress (Contour & Envoy)](#ingress-contour--envoy)
- [logging](#logging)
  - [loki, promtail](#loki-promtail)
- [machines-endpoints](#machines-endpoints)
- [metallb](#metallb)
- [moco](#moco)
- [monitoring](#monitoring)
  - [prometheus, alertmanager, pushgateway](#prometheus-alertmanager-pushgateway)
  - [mackerel-agent](#mackerel-agent)
  - [kube-state-metrics](#kube-state-metrics)
  - [grafana-operator](#grafana-operator)
  - [Grafana](#grafana)
  - [victoriametrics-operator](#victoriametrics-operator)
  - [VictoriaMetrics](#victoriametrics)
- [neco-admission](#neco-admission)
- [network-policy (Calico)](#network-policy-calico)
- [prometheus-adapter](#prometheus-adapter)
- [pvc-autoresizer](#pvc-autoresizer)
- [rook](#rook)
  - [ceph](#ceph)
- [sealed-secrets](#sealed-secrets)
- [teleport](#teleport)
- [topolvm](#topolvm)
- [unbound](#unbound)

## argocd

1. Check [releases](https://github.com/argoproj/argo-cd/releases) for changes.
2. Check [upgrading overview](https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/upgrading/overview.md) when upgrading major or minor version.
3. Run the following command and check the diff.

   ```console
   $ make update-argocd
   $ git diff
   ```

4. Update `KUSTOMIZE_VERSION` in `test/Makefile`.

## cert-manager

Check [the upgrading section](https://cert-manager.io/docs/installation/upgrading/) in the official website.

```console
$ make update-cert-manager
$ git diff
```

## customer-egress

Update the manifests as follows:

```console
$ make update-customer-egress
$ git diff
```

## elastic (ECK)

Check the [Release Notes](https://www.elastic.co/guide/en/cloud-on-k8s/current/eck-release-notes.html) and [Upgrade ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/k8s-upgrading-eck.html) on the official website.

Update the upstream manifests as follows:

```console
$ curl -sLf -o elastic/base/upstream/all-in-one.yaml https://download.elastic.co/downloads/eck/X.Y.Z/all-in-one.yaml
```

Check the difference, and adjust our patches to the new manifests.

## external-dns

Read the following document and fix manifests as necessary.

https://github.com/kubernetes-sigs/external-dns/blob/vX.Y.Z/docs/tutorials/coredns.md

Update the manifests as follows:

```console
$ make update-external-dns
$ git diff
```

## kube-metrics-adapter

Check [releases](https://github.com/zalando-incubator/kube-metrics-adapter/releases).

Update the manifests as follows:

```console
$ make setup   # for the first time to install Helm
$ make update-kube-metrics-adapter
$ git diff kube-metrics-adapter
```

## ingress (Contour & Envoy)

Check the [upgrading guide](https://projectcontour.io/resources/upgrading/) in the official website.

Check diffs of projectcontour/contour files as follows:

```console
$ git clone https://github.com/projectcontour/contour
$ cd contour
$ git checkout vX.Y.Z
$ git diff vA.B.C...vX.Y.Z examples/contour
```

Then, import YAML manifests as follows:

```console
$ cd $GOPATH/src/github.com/cybozu-go/neco-apps
$ rm ./ingress/base/contour/*
$ cp $GOPATH/src/github.com/projectcontour/contour/examples/contour/*.yaml ./ingress/base/contour/
```

Check diffs of contour and envoy deployments as follows:

```console
$ diff -u ingress/base/contour/03-contour.yaml ingress/base/template/deployment-contour.yaml
$ diff -u ingress/base/contour/03-envoy.yaml ingress/base/template/deployment-envoy.yaml
```

Note that:
- We do not use contour's certificate issuance feature, but use cert-manager to issue certificates required for gRPC.
- We change Envoy manifest from DaemonSet to Deployment.
  - We do not create `envoy` service account, and therefore `serviceAccountName: envoy` is removed from Envoy Deployment.
  - We replace or add probes with our custom one bundled in our Envoy container image.
- Not all manifests inherit the upstream. Please check `kustomization.yaml` which manifest inherits or not.
  - If the manifest in the upstream is usable as is, use it from `ingress/base/kustomization.yaml`.
  - If the manifest needs modification:
    - If the manifest is for a cluster-wide resource, put a modified version in the `common` directory.
    - If the manifest is for a namespaced resource, put a template in the `template` directory and apply patches.

## logging

Download Helm used in Loki. Follow `HELM_VERSION` in the upstream configuration.
Grafana does not say helm version requirements explicitly. However, we confirm that the procedure succeeds using helm v3.1.0.
The [document](https://github.com/grafana/helm-charts/tree/main/charts/grafana#upgrading-an-existing-release-to-a-new-major-version) is also helpful.

```console
$ HELM_VERSION=X.Y.Z
$ mkdir -p $GOPATH/src/github.com/cybozu-go/neco-apps/logging/bin
$ curl -sSLf https://get.helm.sh/helm-v$HELM_VERSION-linux-amd64.tar.gz | tar -C $GOPATH/src/github.com/cybozu-go/neco-apps/logging/bin linux-amd64/helm --strip-components 1 -xzf -
```

### loki, promtail
There is no official kubernetes manifests for loki and promtail.
So, check changes in release notes on github and helm charts like bellow.

```
LOGGING_DIR=$GOPATH/src/github.com/cybozu-go/neco-apps/logging
${LOGGING_DIR}/bin/helm repo add grafana https://grafana.github.io/helm-charts

# loki
${LOGGING_DIR}/bin/helm search repo -l grafana | grep grafana/loki
# Choose the latest `CHART VERSION` match with target Loki's `APP VERSION` and set value like below.
LOKI_CHART_VERSION=X.Y.Z
${LOGGING_DIR}/bin/helm template logging --namespace=logging grafana/loki --version=${LOKI_CHART_VERSION} > ${LOGGING_DIR}/base/loki/upstream/loki.yaml

# promtail
${LOGGING_DIR}/bin/helm search repo -l grafana | grep grafana/promtail
# Choose the latest `CHART VERSION` match with target Loki's `APP VERSION` and set value like below.
PROMTAIL_CHART_VERSION=X.Y.Z
${LOGGING_DIR}/bin/helm template logging --namespace=logging grafana/promtail --version=${PROMTAIL_CHART_VERSION} --set rbac.pspEnabled=true > ${LOGGING_DIR}/base/promtail/upstream/promtail.yaml
```

Check the difference between the existing manifest and the new manifest, and update the kustomization patch.
In upstream, loki and promtail settings are stored in secret resource. The configuration is now written in configmap, so decode base64 and compare the settings.

## machines-endpoints

`machines-endpoints` are used in `monitoring` and `bmc-reverse-proxy`.
Update their CronJobs as follows:

```console
$ make update-machines-endpoints
$ git diff
```

## metallb

Check [releases](https://github.com/metallb/metallb/releases)

Update the manifests as follows

```console
$ make update-metallb
$ git diff
```

## moco

Check [releases](https://github.com/cybozu-go/moco/releases) for changes.

Update the manifest as follows:

```console
$ make update-moco
$ git diff
```

## monitoring

### prometheus, alertmanager, pushgateway

There is no official kubernetes manifests for prometheus, alertmanager, and grafana.
So, check changes in release notes on github and helm charts like bellow.

```
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm search repo -l prometheus-community
helm template prom prometheus-community/prometheus --version=11.5.0 > prom-2.18.1.yaml
helm template prom prometheus-community/prometheus --version=11.16.7 > prom-2.21.0.yaml
diff prom-2.18.1.yaml prom-2.21.0.yaml
```

Then edit `monitoring/base/kustomization.yaml` to update the image tags.

Update `PROMTOOL_VERSION` in `test/Makefile`.

### mackerel-agent

mackerel-agent runs only on the real data centers (stage, prod).

Edit the image tags in `monitoring/overlays/*/mackerel-agent.yaml` files.

### kube-state-metrics

Check the manifests in [examples/standard](https://github.com/kubernetes/kube-state-metrics/tree/master/examples/standard) directory.

Update the manifest as follows:

```console
$ make update-kube-state-metrics
$ git diff
```

### grafana-operator

Check [releases](https://github.com/integr8ly/grafana-operator/releases)

Update the manifest as follows:

```console
$ make update-grafana-operator
$ git diff
```

### Grafana

Run the following command.

```yaml
$ make update-grafana
```

### victoriametrics-operator

Check [releases](https://github.com/VictoriaMetrics/operator/releases)

Update the manifest as follows:

```console
$ make update-victoriametrics-operator
$ git diff
```

### VictoriaMetrics

Edit the following files:

- `monitoring/base/victoriametrics/alertmanager.yaml`
  - Update `alertmanager` and `configmap-reload` image tags.
- `monitoring/base/victoriametrics/vmagent-largeset.yaml`
  - Update `victoriametrics-vmagent` and `prometheus-config-reloader` image tags.
- `monitoring/base/victoriametrics/vmagent-smallset.yaml`
  - Update `victoriametrics-vmagent` and `prometheus-config-reloader` image tags.
- `monitoring/base/victoriametrics/vmalert-largeset.yaml`
  - Update `victoriametrics-vmalert` and `configmap-reload` image tags.
- `monitoring/base/victoriametrics/vmalert-smallset.yaml`
  - Update `victoriametrics-vmalert` and `configmap-reload` image tags.
- `monitoring/base/victoriametrics/vmcluster-largeset.yaml`
  - Update `victoriametrics-vmstorage`, `victoriametrics-vmselect`, and `victoriametrics-vminsert` image tags.
- `monitoring/base/victoriametrics/vmsingle-smallset.yaml`
  - Update `victoriametrics-vmsingle` image tag.

## neco-admission

Update the manifest as follows:

```console
$ make update-neco-admission
$ git diff
```

## network-policy (Calico)

Check [the release notes](https://docs.projectcalico.org/release-notes/).

Update the manifest as follows:

```console
$ make update-calico
$ git diff
```

## prometheus-adapter

Check [releases](https://github.com/kubernetes-sigs/prometheus-adapter/releases).

Check the latest Helm chart for prometheus-adapter on https://github.com/prometheus-community/helm-charts .
For example, `prometheus-adapter-2.12.1` is the latest release as of Feb. 28th, 2021.

Update the Helm chart as follows:

```console
$ make update-prometheus-adapter CHART_VERSION=2.12.1
$ git diff
```

## pvc-autoresizer

Check [the CHANGELOG](https://github.com/topolvm/pvc-autoresizer/blob/main/CHANGELOG.md).

Update the manifest as follows:

```console
$ make update-pvc-autoresizer
$ git diff
```

## rook

*Do not upgrade Rook and Ceph at the same time!*

Read [this document](https://github.com/rook/rook/blob/master/Documentation/ceph-upgrade.md) before. Note that you should choose the appropriate release version.

Get upstream helm chart:

```console
$ cd $GOPATH/src/github.com/rook
$ git clone https://github.com/rook/rook
$ cd rook
$ ROOK_VERSION=X.Y.Z
$ git checkout v$ROOK_VERSION
$ ls $GOPATH/src/github.com/cybozu-go/neco-apps/rook/base/upstream/chart
$ rm -rf $GOPATH/src/github.com/cybozu-go/neco-apps/rook/base/upstream/chart
$ cp -a cluster/charts/rook-ceph $GOPATH/src/github.com/cybozu-go/neco-apps/rook/base/upstream/chart
```

Download Helm used in Rook. Follow `HELM_VERSION` in the upstream configuration.

```console
# Check the Helm version, in rook repo directory downloaded above
$ cat $GOPATH/src/github.com/rook/rook/build/makelib/helm.mk | grep ^HELM_VERSION
$ HELM_VERSION=X.Y.Z
$ mkdir -p $GOPATH/src/github.com/cybozu-go/neco-apps/rook/bin
$ curl -sSLf https://get.helm.sh/helm-v$HELM_VERSION-linux-amd64.tar.gz | tar -C $GOPATH/src/github.com/cybozu-go/neco-apps/rook/bin linux-amd64/helm --strip-components 1 -xzf -
```

Update rook/base/values*.yaml if necessary.

Regenerate base resource yaml.  
Note: Check the number of yaml files.

```console
$ cd $GOPATH/src/github.com/cybozu-go/neco-apps/rook/base
$ for i in clusterrole psp resources; do
    ../bin/helm template upstream/chart -f values.yaml -s templates/${i}.yaml > common/${i}.yaml
  done
$ for t in hdd ssd; do
    for i in deployment role rolebinding serviceaccount; do
      ../bin/helm template upstream/chart -f values.yaml -f values-${t}.yaml -s templates/${i}.yaml --namespace ceph-${t} > ceph-${t}/${i}.yaml
    done
    ../bin/helm template upstream/chart -f values.yaml -f values-${t}.yaml -s templates/clusterrolebinding.yaml --namespace ceph-${t} > ceph-${t}/clusterrolebinding/clusterrolebinding.yaml
  done
```

Then check the diffs by `git diff`.

TODO:  
After https://github.com/rook/rook/pull/5573 is merged, we have to revise the above-mentioned process.

Update manifest for Ceph toolbox.
Assume `rook/rook` is updated in the above procedure.

```console
$ cd $GOPATH/src/github.com/cybozu-go/neco-apps/
$ cp $GOPATH/src/github.com/rook/rook/cluster/examples/kubernetes/ceph/toolbox.yaml rook/base/upstream/
```

Update rook/**/kustomization.yaml if necessary.

### ceph

*Do not upgrade Rook and Ceph at the same time!*

Read [this document](https://github.com/rook/rook/blob/master/Documentation/ceph-upgrade.md) first.

Update `spec.cephVersion.image` field in CephCluster CR.

- rook/base/ceph-hdd/cluster.yaml
- rook/base/ceph-ssd/cluster.yaml

## sealed-secrets

Check the [release notes](https://github.com/bitnami-labs/sealed-secrets/blob/master/RELEASE-NOTES.md).

Update the upstream manifests and check the differences as follows:

```console
$ curl -sfL -o sealed-secrets/base/upstream/controller.yaml https://github.com/bitnami-labs/sealed-secrets/releases/download/vX.Y.Z/controller.yaml
$ git diff
```

Then edit `sealed-secrets/base/kustomization.yaml` to update the tag for `quay.io/cybozu/sealed-secrets`.

## teleport

There is no official kubernetes manifests actively maintained for teleport.
So, check changes in [CHANGELOG.md](https://github.com/gravitational/teleport/blob/master/CHANGELOG.md) on github,
and [Helm chart](https://github.com/gravitational/teleport/tree/master/examples/chart/teleport).

```console
$ git clone https://github.com/gravitational/teleport
$ cd teleport
$ git diff vx.y.z...vX.Y.Z examples/chart/teleport
```

Update `TELEPORT_VERSION` in `test/Makefile`.

## topolvm

Check [releases](https://github.com/cybozu-go/topolvm/releases) for changes.

Download the upstream manifest as follows:

```console
$ cd $GOPATH/src/github.com/topolvm
$ git clone https://github.com/topolvm/topolvm
$ cd topolvm
$ git checkout vX.Y.Z
$ cp -r deploy/manifests/* $GOPATH/src/github.com/cybozu-go/neco-apps/topolvm/base/upstream
```

Update `images.newTag` in `kustomization.yaml`.

## unbound

`quay.io/cybozu/unbound` container image is referenced in `customer-egress/base/kustomization.yaml`.
When unbound image is updated, edit the YAML as well.
