package main

import "testing"

func TestValidateStartup(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"localhost no-aws needs nothing", Config{}, false},
		{"no-aws with nothing set", Config{EnableAWS: false}, false},
		{"aws without token", Config{EnableAWS: true, TLSCert: "c", TLSKey: "k"}, true},
		{"aws without tls cert", Config{EnableAWS: true, Token: "t", TLSKey: "k"}, true},
		{"aws without tls key", Config{EnableAWS: true, Token: "t", TLSCert: "c"}, true},
		{"aws with token+tls ok", Config{EnableAWS: true, Token: "t", TLSCert: "c", TLSKey: "k"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateStartup(c.cfg)
			if c.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAWSTargetEntry(t *testing.T) {
	e := AWSTargetEntry()
	if e["name"] != "aws" || e["available"] != false {
		t.Fatalf("aws target entry = %v", e)
	}
	if e["note"] == "" {
		t.Errorf("expected a note field")
	}
}
