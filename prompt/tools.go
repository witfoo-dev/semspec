package prompt

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// ToolGuidance provides one-line guidance for a specific tool.
type ToolGuidance struct {
	// Name is the tool name (e.g., "file_read").
	Name string

	// Guidance is a one-line description of when/how to use this tool.
	Guidance string

	// Roles limits this guidance to specific roles. Empty means all roles.
	Roles []Role

	// Order controls display order in the tool guidance section. Lower values appear first.
	Order int
}

// DefaultToolGuidance returns guidance entries for all semspec tools.
func DefaultToolGuidance() []ToolGuidance {
	return []ToolGuidance{
		// File tools
		{Name: "file_list", Order: 0, Guidance: "List directory contents to discover project structure and find relevant files."},
		{Name: "file_read", Order: 5, Guidance: "Read file contents before modifying. Use to understand existing code patterns and verify current state."},
		{Name: "file_write", Order: 10, Guidance: "Create or modify files. REQUIRED for any code changes — never describe code without writing it.", Roles: []Role{RoleDeveloper}},

		// Git tools
		{Name: "git_status", Order: 20, Guidance: "Check repository status to understand current working tree state."},
		{Name: "git_diff", Order: 25, Guidance: "View changes after modifications to verify correctness before committing."},
		{Name: "git_commit", Order: 30, Guidance: "Commit changes after implementation is complete and verified.", Roles: []Role{RoleDeveloper}},

		// Graph tools — summary first so agents know what to query
		{Name: "graph_summary", Order: 40, Guidance: "What's in the knowledge graph. Call ONCE first to see entity types, domains, and counts."},
		{Name: "graph_search", Order: 42, Guidance: "Search the knowledge graph with a natural language question. Returns a synthesized answer. Use this for any codebase question."},
		{Name: "graph_codebase", Order: 45, Guidance: "Code entity counts and samples. Use after graph_summary for code-specific structure."},
		{Name: "graph_query", Order: 50, Guidance: "Raw GraphQL query for specific lookups: entitiesByPredicate, entity(id), entitiesByPrefix. Use graph_search for general questions."},
		{Name: "read_document", Order: 52, Guidance: "Read plan or specification documents from the workflow."},
		{Name: "graph_entity", Order: 55, Guidance: "Get a specific entity by ID with all predicates."},
		{Name: "graph_traverse", Order: 58, Guidance: "Follow relationships from a known entity (calls, implements, imports). Max depth 3."},

		// Web search
		{Name: "web_search", Order: 60, Guidance: "Search the web for external documentation, API references, or library usage. Max 10 results. Use specific queries, not broad topics."},

		// Advanced tools (reactive execution)
		{Name: "decompose_task", Order: 80, Guidance: "Decompose a task into a DAG of subtasks for parallel execution.", Roles: []Role{RoleDeveloper}},
		{Name: "spawn_agent", Order: 82, Guidance: "Spawn a child agent loop for independent subtask execution.", Roles: []Role{RoleDeveloper}},
		{Name: "create_tool", Order: 84, Guidance: "Create a dynamic tool from a FlowSpec definition.", Roles: []Role{RoleDeveloper}},
		{Name: "query_agent_tree", Order: 86, Guidance: "Inspect the agent hierarchy to understand spawned child agents.", Roles: []Role{RoleDeveloper}},
	}
}

// ToolGuidanceFragment returns a Fragment at CategoryToolGuidance that dynamically
// builds tool guidance from the context's AvailableTools list.
func ToolGuidanceFragment(guidance []ToolGuidance) *Fragment {
	return &Fragment{
		ID:       "core.tool-guidance",
		Category: CategoryToolGuidance,
		Priority: 0,
		Condition: func(ctx *AssemblyContext) bool {
			return len(ctx.AvailableTools) > 1
		},
		ContentFunc: func(ctx *AssemblyContext) string {
			return buildToolGuidanceContent(ctx, guidance)
		},
	}
}

// buildToolGuidanceContent generates the tool guidance section.
func buildToolGuidanceContent(ctx *AssemblyContext, guidance []ToolGuidance) string {
	var sb strings.Builder
	sb.WriteString("Available tools and when to use them:\n\n")

	// Filter to tools available for this role.
	filtered := make([]ToolGuidance, 0, len(guidance))
	for _, g := range guidance {
		if !ctx.HasTool(g.Name) {
			continue
		}
		if len(g.Roles) > 0 && !slices.Contains(g.Roles, ctx.Role) {
			continue
		}
		filtered = append(filtered, g)
	}

	// Sort by Order for consistent, intentional display order.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Order < filtered[j].Order
	})

	for _, g := range filtered {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", g.Name, g.Guidance))
	}

	return sb.String()
}
