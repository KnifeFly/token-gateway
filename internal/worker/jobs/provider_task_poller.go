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

func (j *ProviderTaskPoller) Name() string {
	return "provider_task_poller"
}

func (j *ProviderTaskPoller) Interval() time.Duration {
	return j.interval
}

func (j *ProviderTaskPoller) Timeout() time.Duration {
	return j.timeout
}

func (j *ProviderTaskPoller) MaxConcurrency() int {
	return 1
}

func (j *ProviderTaskPoller) Run(ctx context.Context) error {
	if j == nil || j.repo == nil || j.dispatcher == nil || j.tasks == nil {
		return nil
	}
	tasks, err := j.repo.ListProviderTasks(ctx, j.limit)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		result, err := j.dispatcher.Poll(ctx, task)
		if err != nil {
			return err
		}
		if result == nil || !tasksvc.IsTerminal(result.Status) {
			continue
		}
		if result.Status == tasksvc.StatusSucceeded {
			if err := j.settlement.Settle(ctx, task, result.Usage); err != nil {
				if recordErr := j.settlement.RecordFailed(ctx, task, result.Usage, err); recordErr != nil {
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
