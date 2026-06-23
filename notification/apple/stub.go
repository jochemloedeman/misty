package apple

import "github.com/sideshow/apns2"

type StaticPusher struct {
	StatusCode int
}

func (p StaticPusher) PushWithContext(apns2.Context, *apns2.Notification) (*apns2.Response, error) {
	return &apns2.Response{StatusCode: p.StatusCode}, nil
}
