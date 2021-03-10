monitoring
==========

This directory contains manifests and configuration files for monitoring.
We are using VictoriaMetrics, Grafana, and our own heartbeat client for Opsgenie.

Alert rules
-----------

YAML files for alert rules and tests are placed as follows:

```console
neco-apps
├── monitoring
|   ├── base
|   │   ├── victoriametrics
|   │   |   ├── rules # scraping/alerting rules for each ArgoCD app and particular components (e.g. k8s node)
|   |   |   │   ├── argocd-alertrule.yaml
|   |   |   │   ├── argocd-scrape.yaml
|   |   |   │   ├── ...
|   |   |   │   ├── argocd-alertrule.yaml
|   |   |   │   └── vault-scrape.yaml
|   │   |   └── ...
|   │   └── ...
|   └── overlays
└── test
    ├── vmalert_test # test for each application
    │   ├── argocd.yaml
    │   ├── ...
    │   └── vault.yaml
    └── ...
```

Each YAML file contains tests for the corresponding application.

Run the unit test with the following command:

```console
$ cd $GOPATH/src/github.com/cybozu-go/neco-apps

# Run all tests
$ make test-alert-rules
```

Severity Levels
---------------

All alert rules should have the `severity` labels. This label indicates the level of the severity of the alert.

The severity names and their severity order are consistent with syslog severity. We use just four levels from syslog severity, though.

- `info`: No problem is occurred, but just notify.
- `warning`: Investigate to decide whether any action is required.
- `error`: Action is required, but the situation is not so serious at this time.
- `critical`: Action is required immediately because the problem gets worse. Investigate and resolve the causes of alert as soon as possible. Note: `critical` alerts are intended to be sent to pager even at midnight.

### Critical Alerts

At the moment, the list of `critical` alerts are as follows:

- etcd are missing
  - BootserverEtcdMissing
  - CKEEtcdMissing
- ingress is down
  - ContourGlobalDown
  - IngressGlobalDown
  - ContourForestDown
  - IngressForestDown
  - ContourBastionDown
  - IngressBastionDown
- ingress is down (external probe)
  - IngressDown
  - IngressWatcherDown
- basic kubernetes alerts
  - KubernetesNodesDown
  - KubeControllerManagerDown
  - KubeSchedulerDown
  - KubeAPIErrorsHigh
  - K8sAPIServersDegraded
  - PersistentVolumeSpaceExceeded
- monitoring system failure
  - VMAlertmanagerDown
  - VMAgentSmallsetDown
  - VMAlertSmallsetDown
  - VMSingleSmallsetDown
  - VMAgentLargesetDown
  - VMStorageLargesetDown
  - VMSelectLargesetDown
  - VMInsertLargesetDown
  - AlertmanagerDown
  - PrometheusDown
  - PushGatewayDown
- calico (network-policy)
  - CalicoNodeDown
- rook - the following alerts are marked with `category: storage` label
  - CephHddIsDown
  - CephSsdIsDown
  - RookCephStatusIsError

Notice
------

[Some alert rules](./victoriametrics/rules/kubernetes-alertrule.yaml) come from [coreos/kube-prometheus project](https://github.com/coreos/kube-prometheus).
