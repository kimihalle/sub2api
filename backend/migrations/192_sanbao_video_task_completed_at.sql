ALTER TABLE sanbao_video_tasks ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

WITH completed_source AS (
  SELECT id, NULLIF(response #>> '{data,updated_at}', '') AS response_completed_at
  FROM sanbao_video_tasks
)
UPDATE sanbao_video_tasks t
SET completed_at = COALESCE(
  CASE
    WHEN s.response_completed_at ~ '^\d{4}-\d{2}-\d{2}T' THEN s.response_completed_at::TIMESTAMPTZ
    ELSE NULL
  END,
  t.updated_at
)
FROM completed_source s
WHERE t.id = s.id
  AND t.completed_at IS NULL
  AND t.status IN ('succeeded', 'completed');

CREATE INDEX IF NOT EXISTS idx_sanbao_video_tasks_completed_at ON sanbao_video_tasks(completed_at) WHERE completed_at IS NOT NULL;
