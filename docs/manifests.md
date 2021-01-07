How to write Kubernetes application manifests
=============================================

## Namespace

### Name

- Administrators should not create namespace starting with `app-` as the prefix is reserved for tenants.

### Labels

- All namespaces should have `team=xxx` labels to clarify their owner.
- To skip validation/mutation webhook, administrators can use the following special annotations
  - `mpod.kb.io/ignore`: Use this label in case that the pods in the namespace are required to start in advance of the neco-admission container.
  - `vnetworkpolicy.kb.io/ignore`: Use this label in case that administrators need to set high prioritized network polices for namespaces
  - `topolvm.cybozu.com/webhook`: Use this label in case that the pods in the namespace are required to start in advance of the topolvm containers. See more details [here](https://github.com/topolvm/topolvm/blob/f2726a42fece9ebda70810d42b0ae5fdcb0a6bff/deploy/README.md#protect-system-namespaces-from-topolvm-webhook)

### Annotations

- Namespaces can add the following annotations.
  - `coil.cybozu.com/pool`: Add this annotation to use non-default pools. See more details [here](https://github.com/cybozu-go/coil/blob/4ee51b780fe5a58a05ba9da06f99ca2d2f6693d6/docs/usage.md#using-non-default-pools).



## Secrets

- Administrators should not add sensitive secrets to this repository.
