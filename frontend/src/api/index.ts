import axios from 'axios'

const http = axios.create({ baseURL: '/api', timeout: 20000 })

export type RunStatus = 'pending' | 'running' | 'success' | 'failed' | 'canceled'

export interface PipelineGit {
  url: string
  branch: string
}

export interface PipelineStage {
  name: string
  commands?: string[]
  env?: Record<string, string>
  image?: string
  dockerfile?: string
  context?: string
  push?: boolean
}

export interface Run {
  id: string
  pipeline_name: string
  branch: string
  commit: string
  message: string
  status: RunStatus
  trigger: string
  error: string
  started_at: string | null
  finished_at: string | null
  created_at: string
  stages?: RunStage[]
}

export interface RunStage {
  id: number
  run_id: string
  name: string
  type: string
  status: RunStatus
  exit_code: number
  duration_ms: number
  started_at: string | null
  finished_at: string | null
}

export interface PipelineSummary {
  name: string
  description: string
  git: PipelineGit
  stages: string[]
  updated_at: string
  last_run: Run | null
}

export interface PipelineDetail {
  name: string
  description: string
  git: PipelineGit
  workdir: string
  env: Record<string, string>
  stages: PipelineStage[]
  runs: Run[]
}

export const listPipelines = () => http.get<PipelineSummary[]>('/pipelines').then((r) => r.data)

export const getPipeline = (name: string) =>
  http.get<PipelineDetail>(`/pipelines/${name}`).then((r) => r.data)

export const getPipelineRaw = (name: string) =>
  http.get<string>(`/pipelines/${name}`, { params: { raw: 1 }, responseType: 'text' }).then((r) => r.data)

export const createPipeline = (yamlContent: string) =>
  http.post('/pipelines', { yaml: yamlContent }).then((r) => r.data)

export const updatePipeline = (name: string, yamlContent: string) =>
  http.put(`/pipelines/${name}`, { yaml: yamlContent }).then((r) => r.data)

export const deletePipeline = (name: string) => http.delete(`/pipelines/${name}`)

export const triggerRun = (name: string, branch?: string) =>
  http.post<Run>(`/pipelines/${name}/run`, { branch }).then((r) => r.data)

export const listRuns = (params: { pipeline?: string; page?: number; page_size?: number }) =>
  http.get<{ total: number; items: Run[] }>('/runs', { params }).then((r) => r.data)

export const getRun = (id: string) => http.get<Run>(`/runs/${id}`).then((r) => r.data)

export const cancelRun = (id: string) => http.post(`/runs/${id}/cancel`)

export interface LogChunk {
  text: string
  offset: number
  status: RunStatus
}

export async function fetchLogs(id: string, offset: number): Promise<LogChunk> {
  const res = await http.get(`/runs/${id}/logs`, {
    params: { offset },
    responseType: 'text',
    transformResponse: [(data) => data],
  })
  return {
    text: (res.data as string) ?? '',
    offset: Number(res.headers['x-log-offset'] ?? offset),
    status: (res.headers['x-run-status'] ?? 'running') as RunStatus,
  }
}

export function errorMessage(e: any): string {
  return e?.response?.data?.error ?? e?.message ?? String(e)
}
