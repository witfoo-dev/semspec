/**
 * Types for Scenarios — Given/When/Then behavioral contracts from ADR-024.
 *
 * Scenarios describe observable behavior and belong to Requirements.
 * Tasks satisfy Scenarios in a many-to-many relationship.
 */

// ============================================================================
// Status types
// ============================================================================

/**
 * Scenario status tracking behavioral contract verification.
 * - pending: Not yet verified
 * - passing: Behavioral contract confirmed by execution
 * - failing: Behavioral contract violated
 * - skipped: Intentionally not executed
 */
export type ScenarioStatus = 'pending' | 'passing' | 'failing' | 'skipped';

// ============================================================================
// Core interface
// ============================================================================

/**
 * A behavioral contract (Given/When/Then) that belongs to a Requirement.
 * Tasks satisfy Scenarios (many-to-many).
 */
export interface Scenario {
	id: string;
	requirement_id: string;
	given: string;
	when: string;
	then: string[];
	status: ScenarioStatus;
	created_at: string;
	updated_at: string;
}

// ============================================================================
// Helper functions
// ============================================================================

/**
 * Get display info for a scenario status.
 */
export function getScenarioStatusInfo(status: ScenarioStatus): {
	label: string;
	color: 'gray' | 'green' | 'red' | 'orange';
	icon: string;
} {
	switch (status) {
		case 'pending':
			return { label: 'Pending', color: 'gray', icon: 'circle' };
		case 'passing':
			return { label: 'Passing', color: 'green', icon: 'check-circle' };
		case 'failing':
			return { label: 'Failing', color: 'red', icon: 'x-circle' };
		case 'skipped':
			return { label: 'Skipped', color: 'orange', icon: 'skip-forward' };
	}
}
