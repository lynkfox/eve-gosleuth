package api_test

import (
	"bytes"
	"io"
	"net/http"
)

type MockApiClient struct {
	Method   string
	Endpoint string
	Body     interface{}
	Called   int
	response *http.Response
	err      error
}

func (m MockApiClient) WasCalled() int {
	return m.Called
}

// Mock the Request method
func (m *MockApiClient) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	m.Called = m.Called + 1
	m.Method = method
	m.Endpoint = endpoint
	m.Body = body

	return m.response, m.err
}

func CreateMockResposne(jsonData string, method string, endpoint string, statuscode int) *MockApiClient {

	r := io.NopCloser(bytes.NewReader([]byte(jsonData)))
	return &MockApiClient{
		Method:   method,
		Endpoint: endpoint,
		Body:     nil,
		Called:   0,
		response: &http.Response{
			StatusCode: statuscode,
			Body:       r,
		},
		err: nil,
	}
}
