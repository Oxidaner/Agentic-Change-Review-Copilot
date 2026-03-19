CREATE INDEX IF NOT EXISTS idx_test_tasks_scenario_created_at
    ON test_tasks (scenario, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_test_tasks_status_created_at
    ON test_tasks (status, created_at DESC);
