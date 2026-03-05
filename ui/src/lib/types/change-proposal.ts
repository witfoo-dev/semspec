/**
 * Types for ChangeProposals — mid-stream requirement mutation nodes from ADR-024.
 *
 * A ChangeProposal represents a proposed change to one or more Requirements.
 * On acceptance, the reactive workflow cascades changes to affected Scenarios and Tasks.
 */

// ============================================================================
// Status types
// ============================================================================

/**
 * ChangeProposal lifecycle status.
 * - proposed: Submitted, awaiting review
 * - under_review: Being evaluated
 * - accepted: Approved, cascade in progress or complete
 * - rejected: Declined, no changes made
 * - archived: Historical record, no longer actionable
 */
export type ChangeProposalStatus =
	| 'proposed'
	| 'under_review'
	| 'accepted'
	| 'rejected'
	| 'archived';

// ============================================================================
// Core interface
// ============================================================================

/**
 * A mid-stream change proposal that mutates one or more Requirements.
 */
export interface ChangeProposal {
	id: string;
	plan_id: string;
	title: string;
	rationale: string;
	status: ChangeProposalStatus;
	proposed_by: string;
	affected_requirement_ids: string[];
	created_at: string;
	reviewed_at?: string;
	decided_at?: string;
}

// ============================================================================
// Helper functions
// ============================================================================

/**
 * Get display info for a change proposal status.
 */
export function getChangeProposalStatusInfo(status: ChangeProposalStatus): {
	label: string;
	color: 'blue' | 'orange' | 'green' | 'red' | 'gray';
	icon: string;
} {
	switch (status) {
		case 'proposed':
			return { label: 'Proposed', color: 'blue', icon: 'file-plus' };
		case 'under_review':
			return { label: 'Under Review', color: 'orange', icon: 'eye' };
		case 'accepted':
			return { label: 'Accepted', color: 'green', icon: 'check-circle' };
		case 'rejected':
			return { label: 'Rejected', color: 'red', icon: 'x-circle' };
		case 'archived':
			return { label: 'Archived', color: 'gray', icon: 'archive' };
	}
}
