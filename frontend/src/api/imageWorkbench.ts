import { apiClient } from './client'

export interface ImageWorkbenchSubmitPayload {
  api_key_id: number
  payload: Record<string, unknown>
}

export interface ImageWorkbenchLog {
  id: string
  user_id: number
  api_key_id: number
  api_key_name?: string
  group_id?: number | null
  group_name?: string
  model?: string
  prompt?: string
  status: string
  async: boolean
  task_id?: string
  image_urls?: string[]
  error_message?: string
  request?: Record<string, unknown>
  response?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface ImageWorkbenchLogsResponse {
  data: ImageWorkbenchLog[]
  pagination: {
    page: number
    page_size: number
    total: number
  }
}

export async function generateImage(payload: ImageWorkbenchSubmitPayload): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>('/image-workbench/generations', payload)
  return data
}

export async function listImageWorkbenchLogs(params?: {
  page?: number
  page_size?: number
  status?: string
  model?: string
}): Promise<ImageWorkbenchLogsResponse> {
  const { data } = await apiClient.get<ImageWorkbenchLogsResponse>('/image-workbench/logs', { params })
  return data
}

export async function pollImageTask(taskId: string, apiKeyId: number): Promise<Record<string, unknown>> {
  const { data } = await apiClient.get<Record<string, unknown>>(`/image-workbench/generations/${encodeURIComponent(taskId)}`, {
    params: { api_key_id: apiKeyId }
  })
  return data
}
