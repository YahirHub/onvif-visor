package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultAPIBase = "https://api.telegram.org"

type Client struct {
	httpClient *http.Client
	apiBase    string
}

type BotInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

func New() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 40 * time.Second},
		apiBase:    defaultAPIBase,
	}
}

func NewForTests(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{httpClient: client, apiBase: strings.TrimRight(baseURL, "/")}
}

func (c *Client) GetMe(ctx context.Context, token string) (BotInfo, error) {
	var result BotInfo
	if err := c.callForm(ctx, token, "getMe", nil, &result); err != nil {
		return BotInfo{}, err
	}
	if result.ID == 0 || strings.TrimSpace(result.Username) == "" {
		return BotInfo{}, errors.New("telegram returned an invalid bot identity")
	}
	return result, nil
}

func (c *Client) GetUpdates(ctx context.Context, token string, offset int64) ([]Update, error) {
	values := url.Values{}
	values.Set("offset", strconv.FormatInt(offset, 10))
	values.Set("timeout", "25")
	values.Set("allowed_updates", `["message"]`)
	var result []Update
	if err := c.callForm(ctx, token, "getUpdates", values, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) SendMessage(ctx context.Context, token string, chatID int64, text string) error {
	values := url.Values{}
	values.Set("chat_id", strconv.FormatInt(chatID, 10))
	values.Set("text", text)
	var ignored json.RawMessage
	return c.callForm(ctx, token, "sendMessage", values, &ignored)
}

func (c *Client) SendPhoto(ctx context.Context, token string, chatID int64, image []byte, caption string) error {
	if len(image) == 0 {
		return errors.New("empty telegram photo")
	}
	return c.sendMultipart(ctx, token, "sendPhoto", chatID, "photo", "event.jpg", "image/jpeg", image, caption)
}

func (c *Client) SendVideoFile(ctx context.Context, token string, chatID int64, path, caption string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() <= 0 || info.Size() > 20<<20 {
		return fmt.Errorf("telegram video size out of bounds: %d", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return c.sendMultipart(ctx, token, "sendVideo", chatID, "video", filepath.Base(path), "video/mp4", data, caption)
}

func (c *Client) callForm(ctx context.Context, token, method string, values url.Values, target any) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("telegram token is required")
	}
	if values == nil {
		values = url.Values{}
	}
	endpoint := fmt.Sprintf("%s/bot%s/%s", c.apiBase, token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req, target)
}

func (c *Client) sendMultipart(ctx context.Context, token, method string, chatID int64, field, filename, contentType string, data []byte, caption string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return err
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, sanitizeFilename(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/bot%s/%s", c.apiBase, token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var ignored json.RawMessage
	return c.do(req, &ignored)
}

func (c *Client) do(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return req.Context().Err()
		}
		// No propagar url.Error: la URL contiene el token del bot.
		return errors.New("telegram request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram http status %d", resp.StatusCode)
	}
	var envelope apiResponse[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}
	if !envelope.OK {
		description := strings.TrimSpace(envelope.Description)
		if description == "" {
			description = "telegram api rejected the request"
		}
		return errors.New(description)
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, target); err != nil {
		return fmt.Errorf("decode telegram result: %w", err)
	}
	return nil
}

func ChatDisplayName(chat Chat) string {
	if strings.TrimSpace(chat.Title) != "" {
		return strings.TrimSpace(chat.Title)
	}
	name := strings.TrimSpace(strings.TrimSpace(chat.FirstName) + " " + strings.TrimSpace(chat.LastName))
	if name != "" {
		return name
	}
	if chat.Username != "" {
		return "@" + chat.Username
	}
	return strconv.FormatInt(chat.ID, 10)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	if name == "." || name == "" {
		return "media.bin"
	}
	return name
}
