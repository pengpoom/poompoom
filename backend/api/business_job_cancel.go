package api

import (
	"context"
	"strings"

	"imagestudio/internal/businessjobs"
)

type activeBusinessImageJob struct {
	cancel context.CancelFunc
}

func (s *Server) registerActiveBusinessImageJob(jobID string, cancel context.CancelFunc) func() {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || cancel == nil {
		return func() {}
	}
	s.businessJobMu.Lock()
	if s.activeBusinessJobs == nil {
		s.activeBusinessJobs = map[string]activeBusinessImageJob{}
	}
	s.activeBusinessJobs[jobID] = activeBusinessImageJob{cancel: cancel}
	s.businessJobMu.Unlock()

	return func() {
		s.businessJobMu.Lock()
		delete(s.activeBusinessJobs, jobID)
		s.businessJobMu.Unlock()
	}
}

func (s *Server) cancelActiveBusinessImageJob(jobID string) bool {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return false
	}
	s.businessJobMu.RLock()
	active, ok := s.activeBusinessJobs[jobID]
	s.businessJobMu.RUnlock()
	if !ok || active.cancel == nil {
		return false
	}
	active.cancel()
	return true
}

func businessJobCancelled(job businessjobs.Job) bool {
	status := strings.TrimSpace(job.Status)
	return status == businessjobs.StatusCancelled || status == businessjobs.StatusCancelRequested
}

func businessJobFinal(job businessjobs.Job) bool {
	status := strings.TrimSpace(job.Status)
	return status == businessjobs.StatusCancelled ||
		status == businessjobs.StatusSucceeded ||
		status == businessjobs.StatusFailed
}

func shouldPreserveCurrentBusinessImageJob(current, next businessjobs.Job) bool {
	if !businessJobFinal(current) {
		return false
	}
	if businessJobFinal(next) {
		return false
	}
	return true
}

func (s *Server) businessImageJobByID(jobID string, userID string) (businessjobs.Job, bool) {
	store, err := s.newBusinessJobStore()
	if err != nil {
		return businessjobs.Job{}, false
	}
	defer store.Close()
	job, ok, err := store.Get(context.Background(), jobID, userID)
	if err != nil || !ok {
		return businessjobs.Job{}, false
	}
	return job, true
}
