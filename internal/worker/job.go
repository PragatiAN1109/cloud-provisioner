// Package worker implements a background job queue and worker pool
// that process infrastructure jobs — creating or deleting an
// environment's services — using the existing store and provisioner
// packages.
package worker

// JobType identifies what kind of work a Job represents.
type JobType string

const (
	JobTypeCreate JobType = "CREATE"
	JobTypeDelete JobType = "DELETE"
)

// Job represents one unit of background work: do something (Type) to
// one environment (EnvironmentID). It deliberately holds only the ID,
// not a full Environment snapshot. By the time a worker actually picks
// the job up, the stored environment may have changed since whoever
// submitted the job last looked at it — so the worker always re-fetches
// the current state from the store rather than trusting a potentially
// stale copy carried inside the job itself.
type Job struct {
	Type          JobType
	EnvironmentID string
}
