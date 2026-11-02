package accesslog

import (
	"testing"
	"time"
)

func TestCaddyDecoder_AccessLog(t *testing.T) {
	caddyJSON := `{"level":"info","ts":1793587883.0142,"logger":"http.log.access","msg":"handled request","request":{"remote_ip":"127.0.0.1:52134","client_ip":"127.0.0.1","proto":"HTTP/2.0","method":"GET","host":"api.web.oops","uri":"/v1/users?limit=10","tls":{}},"duration":0.0142,"status":200,"size":1420}`

	decoder := NewCaddyDecoder()
	record, err := decoder.Decode([]byte(caddyJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.IsSystem {
		t.Errorf("expected IsSystem=false, got true")
	}
	if record.Method != "GET" {
		t.Errorf("expected Method=GET, got %s", record.Method)
	}
	if record.Status != 200 {
		t.Errorf("expected Status=200, got %d", record.Status)
	}
	if record.Host != "api.web.oops" {
		t.Errorf("expected Host=api.web.oops, got %s", record.Host)
	}
	if record.Path != "/v1/users?limit=10" {
		t.Errorf("expected Path=/v1/users?limit=10, got %s", record.Path)
	}
	if record.Scheme != "https" {
		t.Errorf("expected Scheme=https, got %s", record.Scheme)
	}
	if record.ClientIP != "127.0.0.1" {
		t.Errorf("expected ClientIP=127.0.0.1, got %s", record.ClientIP)
	}
	expectedLatency := time.Duration(0.0142 * float64(time.Second))
	if record.Latency != expectedLatency {
		t.Errorf("expected Latency=%v, got %v", expectedLatency, record.Latency)
	}
}

func TestCaddyDecoder_SystemLog(t *testing.T) {
	systemJSON := `{"level":"info","ts":1793587883.7299874,"logger":"tls.cache","msg":"replaced certificate in cache","subjects":["httpbin.web.oops"]}`

	decoder := NewCaddyDecoder()
	record, err := decoder.Decode([]byte(systemJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !record.IsSystem {
		t.Errorf("expected IsSystem=true for tls.cache log, got false")
	}
	if record.Level != "INFO" {
		t.Errorf("expected Level=INFO, got %s", record.Level)
	}
	expectedMsg := "[tls.cache] replaced certificate in cache"
	if record.Message != expectedMsg {
		t.Errorf("expected Message=%q, got %q", expectedMsg, record.Message)
	}
}

func TestNginxDecoder_Combined(t *testing.T) {
	line := `127.0.0.1 - - [02/Nov/2026:12:05:01 +0700] "POST /auth/login HTTP/1.1" 502 0 "-" "curl/7.68.0" 0.085`

	decoder := NewNginxDecoder()
	record, err := decoder.Decode([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.IsSystem {
		t.Errorf("expected IsSystem=false, got true")
	}
	if record.Method != "POST" {
		t.Errorf("expected Method=POST, got %s", record.Method)
	}
	if record.Status != 502 {
		t.Errorf("expected Status=502, got %d", record.Status)
	}
	if record.Level != "ERROR" {
		t.Errorf("expected Level=ERROR for 502, got %s", record.Level)
	}
	if record.Path != "/auth/login" {
		t.Errorf("expected Path=/auth/login, got %s", record.Path)
	}
	if record.ClientIP != "127.0.0.1" {
		t.Errorf("expected ClientIP=127.0.0.1, got %s", record.ClientIP)
	}
	expectedLatency := time.Duration(0.085 * float64(time.Second))
	if record.Latency != expectedLatency {
		t.Errorf("expected Latency=%v, got %v", expectedLatency, record.Latency)
	}
}

func TestNginxDecoder_JSON(t *testing.T) {
	line := `{"time_iso8601":"2026-11-02T12:05:01+07:00","remote_addr":"10.0.0.1","http_host":"nginx.web.oops","request_method":"GET","request_uri":"/healthz","status":200,"request_time":"0.003","scheme":"https"}`

	decoder := NewNginxDecoder()
	record, err := decoder.Decode([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.IsSystem {
		t.Errorf("expected IsSystem=false, got true")
	}
	if record.Host != "nginx.web.oops" {
		t.Errorf("expected Host=nginx.web.oops, got %s", record.Host)
	}
	if record.Scheme != "https" {
		t.Errorf("expected Scheme=https, got %s", record.Scheme)
	}
	if record.Status != 200 {
		t.Errorf("expected Status=200, got %d", record.Status)
	}
}

func TestAutoDecoder(t *testing.T) {
	auto := NewAutoDecoder("")

	caddyJSON := `{"level":"info","ts":1793587883,"logger":"http.log.access","msg":"handled request","request":{"method":"GET","host":"web.oops","uri":"/"},"status":200}`
	rec, err := auto.Decode([]byte(caddyJSON))
	if err != nil || rec == nil || rec.Host != "web.oops" {
		t.Errorf("AutoDecoder failed to parse caddy JSON: rec=%v, err=%v", rec, err)
	}

	nginxText := `192.168.1.5 - - [02/Nov/2026:12:05:01 +0700] "GET /docs HTTP/1.1" 200 500`
	rec2, err2 := auto.Decode([]byte(nginxText))
	if err2 != nil || rec2 == nil || rec2.Path != "/docs" {
		t.Errorf("AutoDecoder failed to parse nginx text: rec=%v, err=%v", rec2, err2)
	}
}

