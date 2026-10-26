# Security

## RBAC from the Blueprint

Inrun's runtime and gateway run with the permissions the Blueprint needs and no more: the CRDs it declares, the kinds in each `reconcile.managedResources` or declarative `onCreate`, `onReconcile` and `onDelete` blocks, and what its features use (events, leases, the Secrets it reads).

```bash
inrun generate rbac
```

`inrun validate --full` prints the same rules. A reconciler cannot write a kind the Blueprint did not declare: the runtime refuses it before the API server would.

## TLS

Webhooks need TLS. With no certificate configured, the gateway generates a self-signed one for its Service, keeps it in the `inrun-tls` Secret, writes its CA into the webhook configurations, and rotates it before it expires. With `TLS_CERT` and `TLS_KEY` set, it serves that certificate instead.

Every webhook points at the gateway Service, `security.serviceName.gateway` (default `inrun-gateway`).

## Admission

`admission.validation` and `admission.mutation` on a CRD become a validating and a mutating webhook, served by the gateway. The same rules run at reconcile time, so a CR that slipped past admission (gateway disabled, webhook down, `failurePolicy: Ignore`) is still held to them.

## Conversion

A CRD whose `admission.conversion` declares paths between versions is served by the gateway's `/convert`. With a generated certificate, the gateway also writes the CRD's `spec.conversion` (strategy, Service, CA) and puts it back if it drifts (`conversion.manageCRD`, on by default for the entry that declares the paths). With a certificate you configured, `spec.conversion` is yours to set.

## Protection

- **Deletion protection** (`security.deletionProtection`): a webhook refuses
  to delete protected CRDs and CRs, and Inrun's own namespace, while Inrun
  runs. `failurePolicy: Fail` keeps refusing when the gateway is unreachable;
  `strictMode` also refuses removing the protection label.
- **Namespace protection** (`security.namespaceProtection`): a webhook
  refuses CRs in namespaces a CRD does not allow
  (`runtime.allowedNamespaces` / `restrictedNamespaces`); the runtime enforces
  the same rules when it reconciles.

The gateway's housekeeper keeps the webhook configurations, the TLS Secret
and the CRDs' conversion settings in place, and recreates them if someone
deletes or edits them. `cleanupOnShutdown` removes a webhook when the gateway
stops gracefully.
