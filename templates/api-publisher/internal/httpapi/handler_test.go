package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/api-publisher/internal/task"
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

// newTaskTestServer wires the handler behind a stand-in for the auth
// middleware: only the user_id context key matters here, so we skip real JWKS.
func newTaskTestServer(enq task.Enqueuer) http.Handler {
	th := NewTaskHandler(task.NewService(enq))
	mux := http.NewServeMux()
	stubAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), userIDKey, "user-1")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	mux.Handle("POST /tasks", stubAuth(http.HandlerFunc(th.SubmitTask)))
	return mux
}

func postTask(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestSubmitTask_WhitelistRejectsUnknownTypes(t *testing.T) {
	enq := &captureEnqueuer{}
	h := newTaskTestServer(enq)

	cases := []string{
		`{"routing_key":"mystery.task","type":"mystery.task","payload":{}}`,
		`{"routing_key":"default","type":"default","payload":{}}`,
		`{"routing_key":"example.task","type":"not.a.contract.type","payload":{}}`,
		`{"routing_key":"not.a.contract.type","type":"example.task","payload":{}}`,
	}
	for _, body := range cases {
		w := postTask(t, h, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d (%s)", body, w.Code, w.Body.String())
		}
	}
	if enq.calls != 0 {
		t.Errorf("expected nothing enqueued for rejected types, got %d", enq.calls)
	}
}

func TestSubmitTask_AcceptsContractTypes(t *testing.T) {
	for rk := range task.DefaultRoutingMap {
		enq := &captureEnqueuer{}
		h := newTaskTestServer(enq)

		body, _ := json.Marshal(map[string]interface{}{
			"routing_key": rk,
			"type":        rk,
			"payload":     map[string]interface{}{"k": "v"},
		})
		w := postTask(t, h, string(body))
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
	h := newTaskTestServer(&captureEnqueuer{})
	w := postTask(t, h, `{"payload":{}}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing routing_key/type, got %d", w.Code)
	}
}
