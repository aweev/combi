// internal/services/workflow/saga.go
package workflow

import (
	"go.temporal.io/sdk/workflow"
)

type Saga struct {
	compensations        []func(workflow.Context)
	parallelCompensation bool
}

// NewSaga creates a new Saga instance.
func NewSaga() *Saga {
	return &Saga{
		compensations:        []func(workflow.Context){},
		parallelCompensation: false,
	}
}

func (s *Saga) AddCompensation(fn func(workflow.Context)) {
	s.compensations = append(s.compensations, fn)
}

func (s *Saga) SetParallelCompensation(parallel bool) {
	s.parallelCompensation = parallel
}

func (s *Saga) Compensate(ctx workflow.Context) {
	if len(s.compensations) == 0 {
		return
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting saga compensation", "count", len(s.compensations))

	if s.parallelCompensation {
		// Run all compensations in parallel
		selector := workflow.NewSelector(ctx)
		for i, comp := range s.compensations {
			// Capture loop variable
			compensation := comp
			index := i

			future := workflow.ExecuteLocalActivity(ctx, func(ctx workflow.Context) {
				logger.Info("Executing parallel compensation", "index", index)
				compensation(ctx)
			})

			selector.AddFuture(future, func(f workflow.Future) {
				if err := f.Get(ctx, nil); err != nil {
					logger.Error("Parallel compensation failed", "index", index, "error", err)
				}
			})
		}

		// Wait for all compensations to complete
		for i := 0; i < len(s.compensations); i++ {
			selector.Select(ctx)
		}
	} else {
		// Run compensations in reverse order (LIFO)
		for i := len(s.compensations) - 1; i >= 0; i-- {
			logger.Info("Executing compensation", "index", i, "remaining", i+1)

			// Each compensation function is responsible for handling its own errors
			// We continue with other compensations even if one fails
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Compensation panicked", "index", i, "panic", r)
					}
				}()
				s.compensations[i](ctx)
			}()
		}
	}

	logger.Info("Saga compensation completed")
}

func (s *Saga) Clear() {
	s.compensations = []func(workflow.Context){}
}

func (s *Saga) Count() int {
	return len(s.compensations)
}
