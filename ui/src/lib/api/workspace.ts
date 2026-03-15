import { request } from './client';

export interface WorkspaceTask {
	task_id: string;
	file_count: number;
	branch: string;
}

export interface WorkspaceEntry {
	name: string;
	path: string;
	is_dir: boolean;
	size: number;
	children?: WorkspaceEntry[];
}

export async function fetchWorkspaceTasks(): Promise<WorkspaceTask[]> {
	return request<WorkspaceTask[]>('/workflow-api/workspace/tasks');
}

export async function fetchWorkspaceTree(taskId: string): Promise<WorkspaceEntry[]> {
	return request<WorkspaceEntry[]>(
		`/workflow-api/workspace/tree?task_id=${encodeURIComponent(taskId)}`
	);
}

export async function fetchWorkspaceFile(taskId: string, path: string): Promise<string> {
	const result = await request<{ content: string; size: number }>(
		`/workflow-api/workspace/file?task_id=${encodeURIComponent(taskId)}&path=${encodeURIComponent(path)}`
	);
	return result.content;
}

export function getWorkspaceDownloadUrl(taskId: string): string {
	const base = import.meta.env.VITE_API_URL || '';
	return `${base}/workflow-api/workspace/download?task_id=${encodeURIComponent(taskId)}`;
}
