package alert

import (
	"context"
	"errors"
)

// Combine merges several collectors into one.
//
// Each source is evaluated independently and their alerts are concatenated. A
// failing source does not suppress the others: a routine-load query that times
// out must not hide the fact that a frontend just went down. The combined
// error is returned only when every source failed, so the poller logs one
// outage instead of one line per source.
func Combine(collectors ...CollectFunc) CollectFunc {
	return func(ctx context.Context) ([]Alert, error) {
		var (
			alerts []Alert
			errs   []error
			ok     int
		)

		for _, collect := range collectors {
			found, err := collect(ctx)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			ok++
			alerts = append(alerts, found...)
		}

		if ok == 0 && len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return alerts, nil
	}
}
