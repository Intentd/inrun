package children

// Related objects are attached in their native shape (what the API returns),
// trimmed to the parts expressions read, so KEL's functions work on them
// unchanged and memory stays bounded. A trimmed object keeps the native
// paths: status.containerStatuses is where it is on a Pod.

// trimPod keeps a Pod's identity, ownership, scheduling and full status.
func trimPod(pod map[string]interface{}) map[string]interface{} {
	return trimTo(pod, map[string][]string{
		"metadata": {"name", "namespace", "uid", "labels", "ownerReferences", "creationTimestamp"},
		"spec":     {"nodeName"},
	}, "status")
}

// trimEvent keeps what an Event says and what it is about.
func trimEvent(ev map[string]interface{}) map[string]interface{} {
	out := trimTo(ev, map[string][]string{"metadata": {"name", "namespace"}})
	for _, k := range []string{"type", "reason", "message", "note", "count", "firstTimestamp", "lastTimestamp", "eventTime", "involvedObject", "regarding"} {
		if v, ok := ev[k]; ok {
			out[k] = v
		}
	}
	return out
}

// trimNode keeps a Node's identity, labels (zone, region, instance type),
// taints and status.
func trimNode(node map[string]interface{}) map[string]interface{} {
	return trimTo(node, map[string][]string{
		"metadata": {"name", "labels"},
		"spec":     {"taints", "unschedulable"},
	}, "status")
}

// trimTo copies apiVersion and kind, the listed fields of each listed
// section, and whole the sections named in keep.
func trimTo(obj map[string]interface{}, fields map[string][]string, keep ...string) map[string]interface{} {
	out := map[string]interface{}{}
	for _, k := range []string{"apiVersion", "kind"} {
		if v, ok := obj[k]; ok {
			out[k] = v
		}
	}
	for section, names := range fields {
		src, _ := obj[section].(map[string]interface{})
		if src == nil {
			continue
		}
		dst := map[string]interface{}{}
		for _, n := range names {
			if v, ok := src[n]; ok {
				dst[n] = v
			}
		}
		out[section] = dst
	}
	for _, k := range keep {
		if v, ok := obj[k]; ok {
			out[k] = v
		}
	}
	return out
}
