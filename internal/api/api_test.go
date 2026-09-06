package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/sithukyaw666/watcher/internal/config"
	"github.com/sithukyaw666/watcher/internal/store"
)

type mockStore struct {
	GetAllDeploymentsFunc           func() ([]store.Deployment, error)
	GetLastSuccessfulDeploymentFunc func() (*store.Deployment, error)
}

func (m *mockStore) GetAllDeployments() ([]store.Deployment, error) {
	return m.GetAllDeploymentsFunc()
}

func (m *mockStore) GetLastSuccessfulDeployment() (*store.Deployment, error) {
	return m.GetLastSuccessfulDeploymentFunc()
}

func TestHealthCheck(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	mock := &mockStore{}
	server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))

	t.Run("it should return 200 on /healthz route", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assert.Equal(t, http.StatusOK, response.Code)

	})

	t.Run("it should return 404 on non existence route", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/no-exist", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

}

func TestGetAllDeployment(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("it should return the list of deployment", func(t *testing.T) {
		payload := []store.Deployment{
			{
				ID:            "1",
				CommitHash:    "commit-hash",
				CommitMessage: "initial commit",
				CommitAuthor:  "jondoe",
				Timestamp:     time.Now(),
				Status:        store.StatusSuccess,
			},
		}

		mock := &mockStore{
			GetAllDeploymentsFunc: func() ([]store.Deployment, error) {
				return payload, nil
			},
		}

		var expected bytes.Buffer
		expectedResp := response{
			Message: "Deployment list retrieved successfully",
			Data:    payload,
		}

		json.NewEncoder(&expected).Encode(expectedResp)

		server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))

		request := httptest.NewRequest(http.MethodGet, "/api/deployment/history", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, ResponseContentType, response.Header().Get("Content-Type"))
		assert.JSONEq(t, expected.String(), response.Body.String())
	})

	t.Run("it should return the nil data with status ok ", func(t *testing.T) {

		mock := &mockStore{
			GetAllDeploymentsFunc: func() ([]store.Deployment, error) {
				return nil, nil
			},
		}

		var expected bytes.Buffer
		expectedResp := response{
			Message: "Deployment list is empty",
			Data:    nil,
		}
		json.NewEncoder(&expected).Encode(expectedResp)

		server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))

		request := httptest.NewRequest(http.MethodGet, "/api/deployment/history", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Equal(t, ResponseContentType, response.Header().Get("Content-Type"))
		assert.JSONEq(t, expected.String(), response.Body.String())
	})

	t.Run("it should return the nil data with status no content if something wrong in store layer ", func(t *testing.T) {

		mock := &mockStore{
			GetAllDeploymentsFunc: func() ([]store.Deployment, error) {
				return nil, errors.New("something wrong")
			},
		}

		var expected bytes.Buffer
		expectedResp := response{
			Message: "Cannot get the deployment list",
			Data:    nil,
		}
		json.NewEncoder(&expected).Encode(expectedResp)

		server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))

		request := httptest.NewRequest(http.MethodGet, "/api/deployment/history", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.Equal(t, ResponseContentType, response.Header().Get("Content-Type"))
		assert.JSONEq(t, expected.String(), response.Body.String())
	})

}

func TestGetLastSuccessfulDeployment(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("it should return the last successful deployment", func(t *testing.T) {
		payload := &store.Deployment{
			ID:            "1",
			CommitHash:    "commit-hash",
			CommitMessage: "initial commit",
			CommitAuthor:  "jondoe",
			Timestamp:     time.Now(),
			Status:        store.StatusSuccess,
		}
		mock := &mockStore{
			GetLastSuccessfulDeploymentFunc: func() (*store.Deployment, error) {
				return payload, nil
			},
		}

		var expected bytes.Buffer

		expectedResp := response{
			Message: "Last deployment successfully retrieved",
			Data:    payload,
		}

		json.NewEncoder(&expected).Encode(expectedResp)
		server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))
		request := httptest.NewRequest(http.MethodGet, "/api/deployment/current", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, ResponseContentType, response.Header().Get("Content-Type"))

	})

	t.Run("it should return nil deployment with status ok if none found", func(t *testing.T) {
		mock := &mockStore{
			GetLastSuccessfulDeploymentFunc: func() (*store.Deployment, error) {
				return nil, nil
			},
		}
		var expected bytes.Buffer
		expectedResp := response{
			Message: "No last deployment found",
			Data:    nil,
		}

		json.NewEncoder(&expected).Encode(expectedResp)
		server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))
		request := httptest.NewRequest(http.MethodGet, "/api/deployment/current", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Equal(t, ResponseContentType, response.Header().Get("Content-Type"))

	})

	t.Run("it should return nil deployment with status no content if something wrong in store layer", func(t *testing.T) {
		mock := &mockStore{
			GetLastSuccessfulDeploymentFunc: func() (*store.Deployment, error) {
				return nil, errors.New("something went wrong")
			},
		}
		var expected bytes.Buffer
		expectedResp := response{
			Message: "Cannot get the last deployment",
			Data:    nil,
		}

		json.NewEncoder(&expected).Encode(expectedResp)
		server := NewServer(config.Config{}, mock, logger, make(chan<- struct{}))
		request := httptest.NewRequest(http.MethodGet, "/api/deployment/current", nil)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.Equal(t, ResponseContentType, response.Header().Get("Content-Type"))

	})

}

func TestWebhookCallWithGITHUB(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	c := config.Config{
		WebhookSecret: "It's a Secret to Everybody",
	}
	t.Run("it should send the empty struct into channel on POST ", func(t *testing.T) {
		payload := strings.NewReader("Hello, World!")
		sig := "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"

		triggerChan := make(chan struct{}, 1)
		request := httptest.NewRequest(http.MethodPost, "/api/webhook", payload)
		request.Header.Add(GITHUB_HMAC_HEADER, sig)
		response := httptest.NewRecorder()
		server := NewServer(c, &mockStore{}, logger, triggerChan)
		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusAccepted, response.Code)
		select {
		case in := <-triggerChan:
			assert.NotNil(t, in, "should not be nil")
		case <-time.After(1 * time.Second):
			t.Fatal("timeout: expected to receive data from channel") //
		}
	})

	t.Run("it should return method not allow on GET", func(t *testing.T) {
		triggerChan := make(chan struct{}, 1)
		request := httptest.NewRequest(http.MethodGet, "/api/webhook", nil)
		response := httptest.NewRecorder()
		server := NewServer(c, &mockStore{}, logger, triggerChan)
		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
	})
	t.Run("it should return bad request for webhook call without github headers", func(t *testing.T) {
		triggerChan := make(chan struct{}, 1)
		request := httptest.NewRequest(http.MethodPost, "/api/webhook", nil)
		response := httptest.NewRecorder()
		server := NewServer(c, &mockStore{}, logger, triggerChan)
		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusBadRequest, response.Code)

	})
	t.Run("it should return bad request for webhook call with invalid signature", func(t *testing.T) {
		payload := strings.NewReader("Hello World!")
		sig := "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e78"

		triggerChan := make(chan struct{}, 1)
		request := httptest.NewRequest(http.MethodPost, "/api/webhook", payload)
		request.Header.Add(GITHUB_HMAC_HEADER, sig)
		response := httptest.NewRecorder()
		server := NewServer(c, &mockStore{}, logger, triggerChan)
		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusBadRequest, response.Code)

	})
	t.Run("it should return not found for webhook call without webhook secret configured ", func(t *testing.T) {
		payload := strings.NewReader("Hello World!")
		sig := "hehehe757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e78"

		triggerChan := make(chan struct{}, 1)
		request := httptest.NewRequest(http.MethodPost, "/api/webhook", payload)
		request.Header.Add(GITHUB_HMAC_HEADER, sig)
		response := httptest.NewRecorder()

		server := NewServer(config.Config{}, &mockStore{}, logger, triggerChan)
		server.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)

	})

}
