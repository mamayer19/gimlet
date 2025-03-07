package alert

import (
	"time"

	"github.com/gimlet-io/gimlet-cli/pkg/dashboard/model"
)

type threshold interface {
	isFired() bool
	toAlert() model.Alert
}

type podStrategy struct {
	pod      model.Alert
	waitTime time.Duration
}

type eventStrategy struct {
	event                  model.Alert
	expectedCountPerMinute float64
	expectedCount          int32
}

func (s podStrategy) isFired() bool {
	podLastStateChangeTime := time.Unix(s.pod.LastStateChange, 0)
	waitTime := time.Now().Add(-time.Minute * s.waitTime)

	return podLastStateChangeTime.Before(waitTime)
}

func (s eventStrategy) isFired() bool {
	lastStateChangeInMinutes := time.Since(time.Unix(s.event.LastStateChange, 0)).Minutes()
	countPerMinute := float64(s.event.Count) / lastStateChangeInMinutes

	return countPerMinute >= s.expectedCountPerMinute && s.event.Count >= s.expectedCount
}

func (s podStrategy) toAlert() model.Alert {
	return s.pod
}

func (s eventStrategy) toAlert() model.Alert {
	return s.event
}

func ToThreshold(a *model.Alert, waitTime time.Duration, expectedCount int32, expectedCountPerMinute float64) threshold {
	if a.Type == "pod" {
		return &podStrategy{
			pod:      *a,
			waitTime: waitTime,
		}
	}

	return &eventStrategy{
		event:                  *a,
		expectedCount:          expectedCount,
		expectedCountPerMinute: expectedCountPerMinute,
	}
}
