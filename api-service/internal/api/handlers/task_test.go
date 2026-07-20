package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
)

type captureEnqueuer struct {
	calls int
	lastK string
}

func (m *captureEnqueuer) Enqueue(ctx context.Context, routingKey string, envelope []byte) error {
	m.calls++
	m.lastK = routingKey
	return nil
}

func newTaskTestRouter(enq task.Enqueuer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// Stand-in for the auth middleware: only the context keys matter here.
	engine.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Next()
	})
	h := NewTaskHandler(task.NewService(enq))
	engine.POST("/profiles/:id/tasks", h.SubmitTask)
	return engine
}

func postTask(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/profiles/"+uuid.NewString()+"/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// ADR-008.6: the generic endpoint accepts ONLY the four contract task
// types; everything else is 400 — the default-tasks parking lot is gone.
func TestSubmitTask_WhitelistRejectsUnknownTypes(t *testing.T) {
	enq := &captureEnqueuer{}
	router := newTaskTestRouter(enq)

	cases := []string{
		`{"routing_key":"mystery.task","type":"mystery.task","payload":{}}`,
		`{"routing_key":"default","type":"default","payload":{}}`,
		`{"routing_key":"email.send","type":"not.a.contract.type","payload":{}}`,
		`{"routing_key":"not.a.contract.type","type":"email.send","payload":{}}`,
		`{"routing_key":"document.process.retry.5s","type":"document.process.retry.5s"}`,
	}
	for _, body := range cases {
		w := postTask(t, router, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d (%s)", body, w.Code, w.Body.String())
		}
	}
	if enq.calls != 0 {
		t.Errorf("expected nothing enqueued for rejected types, got %d", enq.calls)
	}
}

func TestSubmitTask_AcceptsAllFourContractTypes(t *testing.T) {
	for rk := range task.DefaultRoutingMap {
		enq := &captureEnqueuer{}
		router := newTaskTestRouter(enq)

		body, _ := json.Marshal(map[string]interface{}{
			"routing_key": rk,
			"type":        rk,
			"payload":     map[string]interface{}{"k": "v"},
		})
		w := postTask(t, router, string(body))
		if w.Code != http.StatusAccepted {
			t.Errorf("rk %s: expected 202, got %d (%s)", rk, w.Code, w.Body.String())
			continue
		}
		if enq.calls != 1 || enq.lastK != rk {
			t.Errorf("rk %s: expected one enqueue on that key, got %d/%s", rk, enq.calls, enq.lastK)
		}
		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp["task_id"] == "" {
			t.Errorf("rk %s: expected task_id in response, got %s", rk, w.Body.String())
		}
	}
}

func TestSubmitTask_MissingFieldsStill400(t *testing.T) {
	router := newTaskTestRouter(&captureEnqueuer{})
	w := postTask(t, router, `{"payload":{}}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing routing_key/type, got %d", w.Code)
	}
}
