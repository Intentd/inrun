// Package protection serves the gateway's protection webhooks:
// /deletion-protection blocks deleting protected CRDs and Inrun's own
// resources, /namespace-protection keeps CRs out of the namespaces their CRD
// forbids, and /strict-mode-protection blocks removing the deletion-protection
// label. The Blueprint is the only way to lift any of them.
package protection
