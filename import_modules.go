package pbgo

import (
	"fmt"
	"net/http"

	"errors"
	"time"

	// "github.com/aws/aws-lambda-go/events"
	// "github.com/aws/aws-lambda-go/lambda"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/singleflight"
)

var ErrInvalidResponse = errors.New("invalid response")

type Client struct {
	client      *resty.Client
	email       string
	password    string
	url         string
	tokenValid  time.Time
	tokenSingle singleflight.Group
}

type (
	authResponse struct {
		Token string `json:"token"`
	}

	Params struct {
		Page      int
		Size      int
		Filters   string
		Sort      string
		Fields    string
		SkipTotal int
	}
)

func NewClient(url, email, password string) *Client {
	client := resty.New()
	client.
		// SetDebug(true).
		SetRetryCount(3).
		SetRetryWaitTime(3 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second)

	return &Client{
		client:      client,
		url:         url,
		email:       email,
		password:    password,
		tokenSingle: singleflight.Group{},
	}
}

func (c *Client) Update(collection, id, origin string, body any) error {
	if err := c.auth(); err != nil {
		return err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"X-Origin":     origin,
		}).
		SetPathParam("collection", collection).
		SetBody(body)

	resp, err := request.Patch(c.url + "/api/collections/{collection}/records/" + id)
	if err != nil {
		return fmt.Errorf("[update] can't send update request to pocketbase, err %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("[update] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return nil
}

func (c *Client) Create(collection, origin string, body any) ([]byte, error) {
	if err := c.auth(); err != nil {
		return nil, err
	}

	request := c.client.R().
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"X-Origin":     origin,
		}).
		SetPathParam("collection", collection).
		SetBody(body)

	resp, err := request.Post(c.url + "/api/collections/{collection}/records")
	if err != nil {
		//print POST request failed
		fmt.Println("POST request failed")
		return nil, err
	}

	if resp.IsError() {
		fmt.Println("Response error code: ", resp.StatusCode())
		fmt.Println("Response error string: ", resp.String())
		return nil, errors.New(resp.String())
	}

	return resp.Body(), nil
}

func (c *Client) Delete(collection, id, origin string) error {
	if err := c.auth(); err != nil {
		return err
	}

	request := c.client.R().
		SetHeaders(map[string]string{
			"Content-Type": "application/json",
			"X-Origin":     origin,
		}).
		SetPathParam("collection", collection).
		SetPathParam("id", id)

	resp, err := request.Delete(c.url + "/api/collections/{collection}/records/{id}")
	if err != nil {
		return fmt.Errorf("[delete] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("[delete] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return nil
}

func (c *Client) List(collection string, params Params) ([]byte, error) {
	if err := c.auth(); err != nil {
		return []byte{}, err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", collection)

	if params.Page > 0 {
		request.SetQueryParam("page", convertor.ToString(params.Page))
	}
	if params.Size > 0 {
		request.SetQueryParam("perPage", convertor.ToString(params.Size))
	}
	if params.Filters != "" {
		request.SetQueryParam("filter", params.Filters)
	}
	if params.Sort != "" {
		request.SetQueryParam("sort", params.Sort)
	}
	if params.Fields != "" {
		request.SetQueryParam("fields", params.Fields)
	}
	if params.SkipTotal == 1 {
		request.SetQueryParam("skipTotal", convertor.ToString(params.SkipTotal))
	}

	resp, err := request.Get(c.url + "/api/collections/{collection}/records")
	if err != nil {
		return []byte{}, fmt.Errorf("[list] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return []byte{}, fmt.Errorf("[list] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return resp.Body(), nil
}

func (c *Client) View(collection string, id string, params Params) ([]byte, error, bool) {
	if err := c.auth(); err != nil {
		return []byte{}, err, false
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam("collection", collection).
		SetPathParam("id", id)

	if params.Fields != "" {
		request.SetQueryParam("fields", params.Fields)
	}

	resp, err := request.Get(c.url + "/api/collections/{collection}/records/{id}")
	if err != nil {
		return []byte{}, fmt.Errorf("[view] can't send update request to pocketbase, err %w", err), false
	}

	if resp.StatusCode() == http.StatusNotFound {
		return []byte{}, fmt.Errorf("%s not found", collection), true
	}

	if resp.IsError() {
		return []byte{}, fmt.Errorf("[view] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		), false
	}

	return resp.Body(), nil, false
}

func (c *Client) ImportCallback(body any) ([]byte, error) {
	if err := c.auth(); err != nil {
		return []byte{}, err
	}

	request := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body)

	resp, err := request.Post(c.url + "/api/v1/import/callback")
	if err != nil {
		return []byte{}, fmt.Errorf("[import/callback] can't send update request to pocketbase, err %w", err)
	}

	if resp.IsError() {
		return []byte{}, fmt.Errorf("[import/callback] pocketbase returned status: %d, msg: %s, err %w",
			resp.StatusCode(),
			resp.String(),
			ErrInvalidResponse,
		)
	}

	return resp.Body(), nil
}

func (c *Client) auth() error {
	_, err, _ := c.tokenSingle.Do("auth", func() (interface{}, error) {
		if time.Now().Before(c.tokenValid) {
			return nil, nil
		}

		resp, err := c.client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(map[string]interface{}{
				"identity": c.email,
				"password": c.password,
			}).
			SetResult(&authResponse{}).
			SetHeader("Authorization", "").
			Post(c.url + "/api/admins/auth-with-password")

		if err != nil {
			return nil, fmt.Errorf("[auth] can't send request to pocketbase %w", err)
		}

		if resp.IsError() {
			return nil, fmt.Errorf("[auth] pocketbase returned status: %d, msg: %s, err %w",
				resp.StatusCode(),
				resp.String(),
				ErrInvalidResponse,
			)
		}

		auth := *resp.Result().(*authResponse)
		c.client.SetHeader("Authorization", auth.Token)
		c.tokenValid = time.Now().Add(60 * time.Minute)

		return nil, nil
	})
	return err
}
