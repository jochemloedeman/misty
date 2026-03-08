package clock

import "time"

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

func (RealClock) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

type FastClock struct {
	inception time.Time
	scale     float64
}

func NewFastClock(scale float64) FastClock {
	return FastClock{
		inception: time.Now(),
		scale:     scale,
	}
}

func (c FastClock) Now() time.Time {
	elapsed := time.Since(c.inception)
	scaledElapsed := time.Duration(float64(elapsed) * c.scale)
	return c.inception.Add(scaledElapsed)
}

func (c FastClock) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(time.Duration(float64(d) * c.scale))
}
