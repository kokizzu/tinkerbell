package worker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

func (c *fakeDockerClient) ImagePull(context.Context, string, image.PullOptions) (io.ReadCloser, error) {
	if c.err != nil {
		return nil, c.err
	}
	return io.NopCloser(strings.NewReader(c.imagePullContent)), nil
}

func (c *fakeDockerClient) ImageInspectWithRaw(context.Context, string) (types.ImageInspect, []byte, error) {
	return types.ImageInspect{}, nil, c.imageInspectErr
}

func TestContainerManagerPullImage(t *testing.T) {
	cases := []struct {
		name            string
		image           string
		responseContent string
		registry        RegistryConnDetails
		clientErr       error
		wantErr         error
		imageInspectErr error
	}{
		{
			name:            "Happy Path",
			image:           "yav.in/4/deathstar:nomedalforchewie",
			responseContent: "{}\n{}",
		},
		{
			name:            "malformed JSON",
			image:           "yav.in/4/deathstar:nomedalforchewie",
			responseContent: "{",
			clientErr:       errors.New("You missed the shot"),
			wantErr:         errors.New("DOCKER PULL: You missed the shot"),
			imageInspectErr: errors.New("Image not in local cache"),
		},
		{
			name:            "pull error",
			image:           "yav.in/4/deathstar:nomedalforchewie",
			responseContent: `{"error": "You missed the shot"}`,
			wantErr:         errors.New("DOCKER PULL: You missed the shot"),
			imageInspectErr: errors.New("Image not in local cache"),
		},
		{
			name:      "image already exists, no error",
			image:     "yav.in/4/deathstar:nomedalforchewie",
			clientErr: errors.New("You missed the shot"),
			wantErr:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := zapr.NewLogger(zap.Must(zap.NewDevelopment()))
			mgr := NewContainerManager(logger, newFakeDockerClient("", tc.responseContent, 0, 0, tc.clientErr, nil, withImageInspectErr(tc.imageInspectErr)), tc.registry)

			ctx := context.Background()
			gotErr := mgr.PullImage(ctx, tc.image)
			if gotErr != nil {
				if tc.wantErr == nil {
					t.Errorf(`Got unexpected error: %v"`, gotErr)
				} else if gotErr.Error() != tc.wantErr.Error() {
					t.Errorf(`Got unexpected error: got "%v" wanted "%v"`, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr == nil && tc.wantErr != nil {
				t.Errorf("Missing expected error: %v", tc.wantErr)
				return
			}
		})
	}
}
