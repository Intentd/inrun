package children

import (
	"context"
	"sort"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// enrichGroupWithCronJobChildren embeds job references under "_activeJobs",
// "_lastJob", and "_lastSuccessfulJob" for each CronJob in the group.
// A no-op when cronjob enrichment is not enabled on the CRD.
func enrichGroupWithCronJobChildren(ctx context.Context, kube kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("cronjob", crd) {
		return
	}
	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		meta, _ := obj["metadata"].(map[string]interface{})
		if meta == nil {
			continue
		}
		ns, _ := meta["namespace"].(string)
		uid, _ := meta["uid"].(string)
		if ns == "" || uid == "" {
			continue
		}

		// _activeJobs: Job references listed in status.active.
		// These are ObjectReferences — {apiVersion, kind, namespace, name, uid}.
		status, _ := obj["status"].(map[string]interface{})
		if status != nil {
			if active, _ := status["active"].([]interface{}); len(active) > 0 {
				obj["_activeJobs"] = active
			}
		}

		// List all Jobs owned by this CronJob for _lastJob and _lastSuccessfulJob.
		list, err := kube.DynamicClient().
			Resource(JobGVR).
			Namespace(ns).
			List(ctx, metav1.ListOptions{ResourceVersion: "0"})
		if err != nil || list == nil {
			continue
		}

		var owned []map[string]interface{}
		for i := range list.Items {
			if objectOwnedByUID(list.Items[i].Object, uid) {
				owned = append(owned, list.Items[i].Object)
			}
		}
		if len(owned) == 0 {
			continue
		}

		// Sort descending by creationTimestamp (RFC3339 string — lexicographic works).
		sort.Slice(owned, func(i, j int) bool {
			return jobCreationTimestamp(owned[i]) > jobCreationTimestamp(owned[j])
		})

		obj["_lastJob"] = owned[0]

		for _, job := range owned {
			jobStatus, _ := job["status"].(map[string]interface{})
			if jobStatus == nil {
				continue
			}
			if toInt64(jobStatus["succeeded"]) > 0 {
				obj["_lastSuccessfulJob"] = job
				break
			}
		}
	}
}

func jobCreationTimestamp(job map[string]interface{}) string {
	meta, _ := job["metadata"].(map[string]interface{})
	if meta == nil {
		return ""
	}
	ts, _ := meta["creationTimestamp"].(string)
	return ts
}
