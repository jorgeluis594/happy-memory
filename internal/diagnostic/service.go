package diagnostic

import "context"

// StoreChecker performs all storage checks without mutating the store.
type StoreChecker interface {
	Check(context.Context, string) ([]Check, error)
}

// Service coordinates global storage diagnostics.
type Service struct {
	path    string
	checker StoreChecker
}

// NewService creates a diagnostic service for one database path.
func NewService(path string, checker StoreChecker) *Service {
	return &Service{path: path, checker: checker}
}

// Doctor returns a report or an unhealthy error containing that report.
func (service *Service) Doctor(ctx context.Context) (Report, error) {
	checks, err := service.checker.Check(ctx, service.path)
	if err != nil {
		return Report{}, err
	}
	if err = validateChecks(checks); err != nil {
		return Report{}, err
	}
	report := Report{Healthy: true, DatabasePath: service.path, Checks: checks}
	for _, check := range checks {
		if !check.OK {
			report.Healthy = false
		}
	}
	if !report.Healthy {
		return report, &UnhealthyError{Report: report}
	}
	return report, nil
}
