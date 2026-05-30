package jobs

import (
	"context"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

// ProviderTaskPoller advances running provider tasks and performs task settlement.
type ProviderTaskPoller struct {
	repo       tasksvc.Repository
	dispatcher tasksvc.ProviderTaskDispatcher
	tasks      *tasksvc.Service
	settlement tasksvc.Settlement
	interval   time.Duration
	timeout    time.Duration
	limit      int
}

// NewProviderTaskPoller returns a provider task poller job.
func NewProviderTaskPoller(repo tasksvc.Repository, dispatcher tasksvc.ProviderTaskDispatcher, tasks *tasksvc.Service, settlement tasksvc.Settlement, interval time.Duration, limit int) *ProviderTaskPoller {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if limit <= 0 {
		limit = 100
	}
	if settlement == nil {
		settlement = tasksvc.NoopSettlement{}
	}
	return &ProviderTaskPoller{
		repo:       repo,
		dispatcher: dispatcher,
		tasks:      tasks,
		settlement: settlement,
		interval:   interval,
		timeout:    30 * time.Second,
		limit:      limit,
	}
}

// Name returns the stable worker job name.
func (j *ProviderTaskPoller) Name() string {
	return "provider_task_poller"
}

// Interval returns how often the job should run.
func (j *ProviderTaskPoller) Interval() time.Duration {
	return j.interval
}

// Timeout returns the maximum runtime for one job execution.
func (j *ProviderTaskPoller) Timeout() time.Duration {
	return j.timeout
}

// MaxConcurrency returns the maximum parallel executions for this job.
func (j *ProviderTaskPoller) MaxConcurrency() int {
	return 1
}

// Run polls provider tasks, settles completed work, and updates task state.
func (j *ProviderTaskPoller) Run(ctx context.Context) error {
	if j == nil || j.repo == nil || j.dispatcher == nil || j.tasks == nil {
		return nil
	}
	tasks, err := j.repo.ListProviderTasks(ctx, j.limit)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		// Step 1: ask the provider adapter for current terminal state.
		result, err := j.dispatcher.Poll(ctx, task)
		if err != nil {
			return err
		}
		if result == nil || !tasksvc.IsTerminal(result.Status) {
			continue
		}
		settlementTask := task
		settlementTask.Status = result.Status
		settlementTask.Result = result.Result
		settlementTask.Usage = result.Usage
		settlementTask.ErrorCode = result.ErrorCode
		settlementTask.ErrorMessage = result.ErrorMessage
		if result.Status == tasksvc.StatusSucceeded {
			// Step 2: settle successful provider work before completing the task.
			if err := j.settlement.Settle(ctx, settlementTask, result.Usage); err != nil {
				if recordErr := j.settlement.RecordFailed(ctx, settlementTask, result.Usage, err); recordErr != nil {
					return recordErr
				}
			}
		}
		if _, err := j.tasks.CompleteTask(ctx, task, *result); err != nil {
			return err
		}
	}
	return nil
}
