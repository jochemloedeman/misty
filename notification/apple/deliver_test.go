package apple

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/notification"
	"github.com/sideshow/apns2"
)

const testToken = "device-token-abc"

type stubResolver struct {
	token    string
	tokenErr error
	clearErr error

	clearCalled  bool
	clearedToken string
}

func (s *stubResolver) PushToken(context.Context, uuid.UUID) (string, error) {
	return s.token, s.tokenErr
}

func (s *stubResolver) ClearPushToken(_ context.Context, _ uuid.UUID, token string) error {
	s.clearCalled = true
	s.clearedToken = token
	return s.clearErr
}

type fakePusher struct {
	resp *apns2.Response
	err  error

	pushCalled bool
}

func (f *fakePusher) PushWithContext(apns2.Context, *apns2.Notification) (*apns2.Response, error) {
	f.pushCalled = true
	return f.resp, f.err
}

func TestNewDeliverer(t *testing.T) {
	tests := []struct {
		name        string
		resolver    stubResolver
		pusher      fakePusher
		wantErr     bool
		wantCleared bool
		wantPushed  bool
	}{
		{
			name:        "410 clears token",
			resolver:    stubResolver{token: testToken},
			pusher:      fakePusher{resp: &apns2.Response{StatusCode: 410}},
			wantCleared: true,
			wantPushed:  true,
		},
		{
			name:        "410 clear fails",
			resolver:    stubResolver{token: testToken, clearErr: errors.New("clear failed")},
			pusher:      fakePusher{resp: &apns2.Response{StatusCode: 410}},
			wantErr:     true,
			wantCleared: true,
			wantPushed:  true,
		},
		{
			name:       "success 200",
			resolver:   stubResolver{token: testToken},
			pusher:     fakePusher{resp: &apns2.Response{StatusCode: 200}},
			wantPushed: true,
		},
		{
			name:       "rejected 400",
			resolver:   stubResolver{token: testToken},
			pusher:     fakePusher{resp: &apns2.Response{StatusCode: 400, Reason: "BadDeviceToken"}},
			wantErr:    true,
			wantPushed: true,
		},
		{
			name:     "empty token skips",
			resolver: stubResolver{token: ""},
		},
		{
			name:     "resolve error",
			resolver: stubResolver{tokenErr: errors.New("db down")},
			wantErr:  true,
		},
		{
			name:       "push transport error",
			resolver:   stubResolver{token: testToken},
			pusher:     fakePusher{err: errors.New("connection refused")},
			wantErr:    true,
			wantPushed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := tt.resolver
			pusher := tt.pusher
			deliver := NewDeliverer(&pusher, &resolver, "com.example.app")

			notif := notification.Fog{ID: uuid.New(), RecipientID: uuid.New()}
			err := deliver(t.Context(), notif)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if resolver.clearCalled != tt.wantCleared {
				t.Errorf("clearCalled = %v, want %v", resolver.clearCalled, tt.wantCleared)
			}
			if tt.wantCleared && resolver.clearedToken != testToken {
				t.Errorf("clearedToken = %q, want %q", resolver.clearedToken, testToken)
			}
			if pusher.pushCalled != tt.wantPushed {
				t.Errorf("pushCalled = %v, want %v", pusher.pushCalled, tt.wantPushed)
			}
		})
	}
}
