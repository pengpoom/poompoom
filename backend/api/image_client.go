package api

import (
	"context"

	"imagestudio/handler"
)

type imageDownloader interface {
	DownloadBytes(url string) ([]byte, error)
	DownloadAsBase64(ctx context.Context, url string) (string, error)
}

type imageWorkflowClient interface {
	handler.ImageWorkflowClient
}

type cpaRouteAwareImageWorkflowClient interface {
	imageWorkflowClient
	LastRoute() string
	LastModelLabel() string
}
