package main

import "testing"

// Captured shape of `docker compose -p microservices ps --format json`:
// one JSON object per line.
const composePSFixture = `{"ID":"1a2b","Name":"microservices-api-service-1","Service":"api-service","State":"running","Health":"healthy","Image":"microservices/api-service:latest","Status":"Up 2 hours (healthy)"}
{"ID":"3c4d","Name":"microservices-auth-service-1","Service":"auth-service","State":"running","Health":"healthy","Image":"microservices/auth-service:latest","Status":"Up 2 hours (healthy)"}
{"ID":"5e6f","Name":"microservices-graphrag-service-1","Service":"graphrag-service","State":"restarting","Health":"unhealthy","Image":"microservices/graphrag-service:latest","Status":"Restarting (1) 5 seconds ago"}
{"ID":"7a8b","Name":"microservices-email-worker-1","Service":"email-worker","State":"running","Health":"","Image":"microservices/email-worker:latest","Status":"Up 2 hours"}
{"ID":"9c0d","Name":"microservices-postgres-1","Service":"postgres","State":"exited","Health":"","Image":"postgres:16","Status":"Exited (0) 1 hour ago"}`

func TestSummarizeComposePS(t *testing.T) {
	services, err := summarizeComposePS([]byte(composePSFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 5 {
		t.Fatalf("expected 5 services, got %d", len(services))
	}

	want := []ComposeService{
		{Name: "api-service", State: "running", Health: "healthy", Image: "microservices/api-service:latest"},
		{Name: "auth-service", State: "running", Health: "healthy", Image: "microservices/auth-service:latest"},
		{Name: "graphrag-service", State: "restarting", Health: "unhealthy", Image: "microservices/graphrag-service:latest"},
		{Name: "email-worker", State: "running", Health: "none", Image: "microservices/email-worker:latest"},
		{Name: "postgres", State: "exited", Health: "none", Image: "postgres:16"},
	}
	for i, w := range want {
		if services[i] != w {
			t.Errorf("service %d: got %+v, want %+v", i, services[i], w)
		}
	}
}

func TestSummarizeComposePSArrayForm(t *testing.T) {
	// Older docker versions emit a single JSON array.
	raw := `[{"Name":"microservices-api-service-1","Service":"api-service","State":"running","Health":"healthy","Image":"img:1"}]`
	services, err := summarizeComposePS([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 || services[0].Name != "api-service" {
		t.Fatalf("got %+v", services)
	}
}

func TestSummarizeComposePSEmpty(t *testing.T) {
	services, err := summarizeComposePS([]byte("\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 0 {
		t.Fatalf("expected no services, got %d", len(services))
	}
}

func TestSummarizeComposePSMalformed(t *testing.T) {
	if _, err := summarizeComposePS([]byte("not json")); err == nil {
		t.Fatal("expected error for malformed input")
	}
}

func TestComposeProjectListed(t *testing.T) {
	raw := `[{"Name":"microservices","Status":"running(6)","ConfigFiles":"/repo/docker-compose.yml"},{"Name":"other","Status":"running(1)","ConfigFiles":"/x/docker-compose.yml"}]`
	if !composeProjectListed([]byte(raw), "microservices") {
		t.Error("expected microservices to be listed")
	}
	if composeProjectListed([]byte(raw), "absent") {
		t.Error("did not expect absent project to be listed")
	}
	if composeProjectListed([]byte("[]"), "microservices") {
		t.Error("did not expect project in empty list")
	}
	if composeProjectListed([]byte("garbage"), "microservices") {
		t.Error("did not expect project in malformed list")
	}
}
