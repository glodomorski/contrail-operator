package job

import (
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

type Status batch.JobStatus

func (s Status) Pending() bool {
	if len(s.Conditions) == 0 {
		return true
	}
	for _, condition := range s.Conditions {
		if condition.Status == v1.ConditionTrue && (condition.Type == batch.JobComplete || condition.Type == batch.JobFailed) {
			return false
		}
	}
	return true
}
