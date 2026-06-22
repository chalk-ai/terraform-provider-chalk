package provider

import (
	"errors"
	"fmt"
)

// Lifecycle status strings reported by the CloudComponents API on the `status`
// field of managed VPC and cluster responses. Async create/update/delete
// operations progress through PENDING/PROVISIONING/UPDATING/DELETING to the
// terminal ACTIVE/DELETED states, or FAILED. Unrecognized values are treated
// as a non-terminal, in-progress state.
const (
	cloudComponentStatusActive  = "ACTIVE"
	cloudComponentStatusFailed  = "FAILED"
	cloudComponentStatusDeleted = "DELETED"
)

// deletionComplete evaluates a managed cloud component's lifecycle status while
// it is being torn down. `found` reports whether the component could still be
// fetched (i.e. Get did not return not-found).
//
// It returns done=true once the component has finished deleting:
//   - the component can no longer be fetched (found=false), or
//   - it reports the terminal DELETED status.
//
// A FAILED status returns a non-nil failure (teardown failed). Any other
// status (notably DELETING) means teardown is still in progress, so the caller
// should keep polling.
func deletionComplete(found bool, status, statusError string) (done bool, failure error) {
	if !found {
		return true, nil
	}
	switch status {
	case cloudComponentStatusDeleted:
		return true, nil
	case cloudComponentStatusFailed:
		if statusError != "" {
			return true, fmt.Errorf("reported status %s: %s", cloudComponentStatusFailed, statusError)
		}
		return true, errors.New("reported status " + cloudComponentStatusFailed)
	default:
		return false, nil
	}
}

// terminalStatus evaluates a managed cloud component's lifecycle status.
//
// It returns terminal=true once the component has reached a terminal state:
//   - ACTIVE: a healthy create/update completed; failure is nil.
//   - FAILED: the operation failed; failure carries the status_error detail.
//
// Any other (in-flight or unrecognized) status returns terminal=false, meaning
// the caller should keep polling.
func terminalStatus(status, statusError string) (terminal bool, failure error) {
	switch status {
	case cloudComponentStatusActive:
		return true, nil
	case cloudComponentStatusFailed:
		if statusError != "" {
			return true, fmt.Errorf("reported status %s: %s", cloudComponentStatusFailed, statusError)
		}
		return true, errors.New("reported status " + cloudComponentStatusFailed)
	default:
		return false, nil
	}
}
