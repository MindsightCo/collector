package apiclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	prommodel "github.com/prometheus/common/model"
)

type TokenBuilder interface {
	GetAccessToken() (string, error)
}

type MetricsPusher struct {
	url   string
	grant TokenBuilder
}

func (p *MetricsPusher) Push(metrics map[int]prommodel.Vector) error {
	token, err := p.grant.GetAccessToken()
	if err != nil {
		return errors.Wrap(err, "get access token:")
	}

	payload, err := json.Marshal(metrics)
	if err != nil {
		return errors.Wrap(err, "json marshal metrics")
	}

	req, err := http.NewRequest("POST", p.url, bytes.NewBuffer(payload))
	if err != nil {
		return errors.Wrap(err, "create http request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "do http request")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("response status: %s, body: %s", resp.Status, string(body))
	}

	return nil
}
