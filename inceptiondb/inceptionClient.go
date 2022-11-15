package inceptiondb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Config struct {
	Base       string `json:"base"`
	DatabaseID string `json:"database_id"`
	ApiKey     string `json:"api_key"`
	ApiSecret  string `json:"api_secret"`
}

var DefaultHttpClient = &http.Client{} // TODO: tune client

type Client struct {
	config     Config
	HttpClient *http.Client
}

type JSON = map[string]interface{}

func NewClient(config Config) *Client {
	return &Client{
		config:     config,
		HttpClient: DefaultHttpClient, // set default http client
	}
}

var ErrorAlreadyExist = errors.New("already exists")
var ErrorForbidden = errors.New("forbidden, your credentials does not have access")
var ErrorUnauthorized = errors.New("unauthorized, missing credentials")
var ErrorUnexpected = errors.New("unexpected error")

func (c *Client) CreateCollection(name string) error {
	endpoint := c.config.Base + "/databases/" + c.config.DatabaseID + "/collections"

	payload, err := json.Marshal(JSON{
		"name": name,
	}) // todo: handle err
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Api-Key", c.config.ApiKey)
	req.Header.Set("Api-Secret", c.config.ApiSecret)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusCreated {
		return nil
	}

	// body, _ := io.ReadAll(resp.Body)
	// log.Println("CreateCollection: " + string(body))

	if resp.StatusCode == http.StatusConflict {
		return ErrorAlreadyExist
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrorForbidden
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrorUnauthorized
	}

	return ErrorUnexpected
}

func (c *Client) EnsureCollection(name string) error {

	c.CreateCollection(name)
	// todo: return nil only if conflict or created
	return nil
}

func (c *Client) Insert(collection string, document interface{}) error {

	payload, err := json.Marshal(document) // todo: handle err
	if err != nil {
		return err
	}

	endpoint := c.config.Base + "/databases/" + url.PathEscape(c.config.DatabaseID) + "/collections/" + url.PathEscape(collection) + ":insert"
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", c.config.ApiKey)
	req.Header.Set("Api-Secret", c.config.ApiSecret)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}

	io.Copy(io.Discard, resp.Body)
	// todo: unmarshal body on document?

	if resp.StatusCode == http.StatusCreated {
		return nil
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrorForbidden
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrorUnauthorized
	}

	return ErrorUnexpected
}

type FindQuery struct {
	Index   string `json:"index"`
	Skip    int    `json:"skip"`
	Limit   int    `json:"limit"`
	Filter  JSON   `json:"filter"`
	Reverse bool   `json:"reverse"`
}

func (c *Client) Find(collection string, query FindQuery) (io.Reader, error) {

	payload, err := json.Marshal(query)
	if err != nil {
		return nil, err // todo: wrap error
	}

	endpoint := c.config.Base + "/databases/" + url.PathEscape(c.config.DatabaseID) + "/collections/" + url.PathEscape(collection) + ":find"

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", c.config.ApiKey)
	req.Header.Set("Api-Secret", c.config.ApiSecret)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("persistence read error")
	}

	if resp.StatusCode == http.StatusOK {
		return resp.Body, nil
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrorForbidden
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrorUnauthorized
	}

	return nil, ErrorUnexpected
}

type GetCollectionInfo struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

func (c *Client) GetCollection(collection string) (*GetCollectionInfo, error) {

	endpoint := c.config.Base + "/databases/" + url.PathEscape(c.config.DatabaseID) + "/collections/" + url.PathEscape(collection)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Api-Key", c.config.ApiKey)
	req.Header.Set("Api-Secret", c.config.ApiSecret)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("persistence read error")
	}

	if resp.StatusCode == http.StatusOK {
		output := &GetCollectionInfo{}
		err := json.NewDecoder(resp.Body).Decode(&output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrorForbidden
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrorUnauthorized
	}

	return nil, ErrorUnexpected
}
