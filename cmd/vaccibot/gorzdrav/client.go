package gorzdrav

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	http *http.Client
	rl   *rate.Limiter
}

func NewClient() *Client {
	return &Client{
		rl: rate.NewLimiter(rate.Every(time.Second), 1),
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	if err := c.rl.Wait(ctx); err != nil {
		return nil, err
	}

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error doing get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body %s: %w", url, err)
	}

	return body, nil
}

func (c *Client) GetDistricts(ctx context.Context) ([]*District, error) {
	body, err := c.get(ctx, "https://gorzdrav.spb.ru/_api/api/district")
	if err != nil {
		return nil, fmt.Errorf("error requesting districts: %w", err)
	}

	resp := &ResponseDistrict{}
	if err := json.Unmarshal(body, resp); err != nil {
		return nil, fmt.Errorf("error unmarshaling districts: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("fail response for districts: %s", body)
	}

	return resp.Result, nil
}

func (c *Client) GetLPUs(ctx context.Context, district *District) ([]*LPU, error) {
	body, err := c.get(ctx, fmt.Sprintf(
		"https://gorzdrav.spb.ru/_api/api/district/%s/lpu?covidVaccination=true", district.ID,
	))
	if err != nil {
		return nil, fmt.Errorf("error requesting LPUs %s: %w", district.ID, err)
	}

	resp := &ResponseLPU{}
	if err := json.Unmarshal(body, resp); err != nil {
		return nil, fmt.Errorf("error unmarshaling LPUs %s: %w", district.ID, err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("fail response for LPUs %s: %s", district.ID, body)
	}

	for _, lpu := range resp.Result {
		lpu.District = district
	}

	return resp.Result, nil
}

func (c *Client) GetSpecialties(ctx context.Context, lpu *LPU) ([]*Specialty, error) {
	body, err := c.get(ctx, fmt.Sprintf(
		"https://gorzdrav.spb.ru/_api/api/lpu/%d/speciality", lpu.ID,
	))
	if err != nil {
		return nil, fmt.Errorf("error requesting specialties %d: %w", lpu.ID, err)
	}

	resp := &ResponseSpecialty{}
	if err := json.Unmarshal(body, resp); err != nil {
		return nil, fmt.Errorf("error unmarshaling specialties %d: %w", lpu.ID, err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("fail response for specialties %d: %s", lpu.ID, body)
	}

	for _, specialty := range resp.Result {
		specialty.LPU = lpu
	}

	return resp.Result, nil
}
