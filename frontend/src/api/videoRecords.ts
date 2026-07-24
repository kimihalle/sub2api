import { apiClient } from './client'

export interface VideoRecord {
  id: number
  task_id: string
  user_id: number
  api_key_id: number
  api_key_name?: string
  account_id: number
  account_name?: string
  group_id?: number | null
  group_name?: string
  model: string
  upstream_model?: string
  prompt?: string
  status: string
  ratio?: string
  resolution?: string
  duration_seconds?: number
  video_url?: string
  download_url?: string
  error_message?: string
  elapsed_seconds?: number
  cost: number
  refund_amount?: number | null
  refunded_at?: string | null
  completed_at?: string | null
  created_at: string
  updated_at: string
}

export interface VideoRecordsResponse {
  data: VideoRecord[]
  pagination: {
    page: number
    page_size: number
    total: number
  }
}

export async function listVideoRecords(params?: {
  page?: number
  page_size?: number
  status?: string
  model?: string
}): Promise<VideoRecordsResponse> {
  const { data } = await apiClient.get<VideoRecordsResponse>('/video-records', { params })
  return data
}

export async function fetchVideoRecordContent(taskId: string, download = false): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/video-records/${encodeURIComponent(taskId)}/content`, {
    params: download ? { download: 1 } : undefined,
    responseType: 'blob',
    timeout: 120000,
  })
  return data
}
