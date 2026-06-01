package main

import "testing"

func TestResolveAdvertiseAddr_Empty(t *testing.T) {
	result := resolveAdvertiseAddr("", "8080")
	expected := "127.0.0.1:8080"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_EmptyCustomPort(t *testing.T) {
	result := resolveAdvertiseAddr("", "9090")
	expected := "127.0.0.1:9090"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_NoPort(t *testing.T) {
	result := resolveAdvertiseAddr("10.0.1.5", "8080")
	expected := "10.0.1.5:8080"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_NoPortHostname(t *testing.T) {
	result := resolveAdvertiseAddr("scheduler-node1", "50051")
	expected := "scheduler-node1:50051"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_WithMatchingPort(t *testing.T) {
	result := resolveAdvertiseAddr("10.0.1.5:8080", "8080")
	expected := "10.0.1.5:8080"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_WithDifferentPort(t *testing.T) {
	result := resolveAdvertiseAddr("10.0.1.5:80", "8080")
	expected := "10.0.1.5:80"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_IPv6WithPort(t *testing.T) {
	result := resolveAdvertiseAddr("[::1]:8080", "8080")
	expected := "[::1]:8080"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveAdvertiseAddr_InvalidFormat(t *testing.T) {
	result := resolveAdvertiseAddr(":::bad", "8080")
	if result != ":::bad" {
		t.Errorf("expected %q, got %q", ":::bad", result)
	}
}
