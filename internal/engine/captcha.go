package engine

import "context"

type CaptchaSolver interface {
    Solve(ctx context.Context, img []byte, captchaType string) (*CaptchaResult, error)
    ReportError(ctx context.Context, id string) error
}

type CaptchaResult struct {
    ID   string
    Code string
}
