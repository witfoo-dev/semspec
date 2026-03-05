package workflow

import (
	"testing"
)

func TestTaskStatus_IsValid_NewStatuses(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   bool
	}{
		{TaskStatusBlocked, true},
		{TaskStatusDirty, true},
		// Existing statuses still valid
		{TaskStatusPending, true},
		{TaskStatusPendingApproval, true},
		{TaskStatusApproved, true},
		{TaskStatusRejected, true},
		{TaskStatusInProgress, true},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
		// Invalid
		{"", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("TaskStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestTaskStatus_CanTransitionTo_BlockedAndDirty(t *testing.T) {
	tests := []struct {
		from TaskStatus
		to   TaskStatus
		want bool
	}{
		// approved -> blocked
		{TaskStatusApproved, TaskStatusBlocked, true},
		// approved -> dirty
		{TaskStatusApproved, TaskStatusDirty, true},
		// blocked -> in_progress (dependency resolved)
		{TaskStatusBlocked, TaskStatusInProgress, true},
		// blocked -> dirty (upstream changed while blocked)
		{TaskStatusBlocked, TaskStatusDirty, true},
		// blocked -> approved (invalid)
		{TaskStatusBlocked, TaskStatusApproved, false},
		// dirty -> pending_approval (re-evaluate)
		{TaskStatusDirty, TaskStatusPendingApproval, true},
		// dirty -> approved (invalid)
		{TaskStatusDirty, TaskStatusApproved, false},
		// dirty -> in_progress (invalid)
		{TaskStatusDirty, TaskStatusInProgress, false},
		// pending -> dirty
		{TaskStatusPending, TaskStatusDirty, true},
		// pending_approval -> dirty
		{TaskStatusPendingApproval, TaskStatusDirty, true},
		// rejected -> dirty
		{TaskStatusRejected, TaskStatusDirty, true},
		// completed is terminal (no dirty)
		{TaskStatusCompleted, TaskStatusDirty, false},
		// failed is terminal (no dirty)
		{TaskStatusFailed, TaskStatusDirty, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("(%q).CanTransitionTo(%q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
