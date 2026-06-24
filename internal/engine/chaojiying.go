package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type ChaojiyingSolver struct {
	user    string
	pass    string
	softID  string
	client  *http.Client
	baseURL string
}

type chaojiyingResp struct {
	ErrNo  int    `json:"err_no"`
	ErrStr string `json:"err_str"`
	PicID  string `json:"pic_id"`
	PicStr string `json:"pic_str"`
}

func NewChaojiyingSolver(user, pass, softID string) *ChaojiyingSolver {
	return &ChaojiyingSolver{
		user:    user,
		pass:    pass,
		softID:  softID,
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "http://upload.chaojiying.net/Upload/Processing.php",
	}
}

func (s *ChaojiyingSolver) Solve(ctx context.Context, img []byte, captchaType string) (*CaptchaResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("user", s.user)
	w.WriteField("pass", s.pass)
	w.WriteField("softid", s.softID)
	w.WriteField("codetype", captchaType)
	w.WriteField("len_min", "0")
	w.WriteField("time_out", "20")
	fw, _ := w.CreateFormFile("userfile", "captcha.png")
	fw.Write(img)
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chaojiying request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r chaojiyingResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("chaojiying parse: %w", err)
	}
	if r.ErrNo != 0 {
		return nil, fmt.Errorf("chaojiying error %d: %s", r.ErrNo, r.ErrStr)
	}
	return &CaptchaResult{ID: r.PicID, Code: r.PicStr}, nil
}

func (s *ChaojiyingSolver) ReportError(ctx context.Context, id string) error {
	url := fmt.Sprintf("http://upload.chaojiying.net/Upload/ReportBad.php?action=reportbad&id=%s&softid=%s", id, s.softID)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
