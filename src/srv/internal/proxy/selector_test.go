package proxy

import (
	"net/http"
	"testing"
)

func cands() []ResolvedProxy {
	return []ResolvedProxy{
		{ID: "a", URL: "http://a", Priority: 0},
		{ID: "b", URL: "http://b", Priority: 1},
		{ID: "c", URL: "http://c", Priority: 2},
	}
}

func TestFailoverPicksPrimaryThenAdvances(t *testing.T) {
	s := NewSelector("failover")
	got, ok := s.Pick(cands())
	if !ok || got.ID != "a" {
		t.Fatalf("expected primary a, got %+v ok=%v", got, ok)
	}
	// mark a bad → should advance to b
	s.MarkResult("a", false)
	got, _ = s.Pick(cands())
	if got.ID != "b" {
		t.Fatalf("expected b after a fails, got %s", got.ID)
	}
	// success on b clears nothing relevant; a stays bad
	got, _ = s.Pick(cands())
	if got.ID != "b" {
		t.Fatalf("expected b still, got %s", got.ID)
	}
	// success on a restores it as primary
	s.MarkResult("a", true)
	got, _ = s.Pick(cands())
	if got.ID != "a" {
		t.Fatalf("expected a restored as primary, got %s", got.ID)
	}
}

func TestFailoverAllBadResetsToPrimary(t *testing.T) {
	s := NewSelector("failover")
	s.MarkResult("a", false)
	s.MarkResult("b", false)
	s.MarkResult("c", false)
	got, ok := s.Pick(cands())
	if !ok || got.ID != "a" {
		t.Fatalf("expected reset to primary a, got %+v", got)
	}
}

func TestFailoverEmpty(t *testing.T) {
	s := NewSelector("failover")
	if _, ok := s.Pick(nil); ok {
		t.Fatal("expected no pick on empty candidates")
	}
}

func TestRoundRobinCycles(t *testing.T) {
	s := NewSelector("round_robin")
	cs := cands()
	seen := map[string]int{}
	for i := 0; i < 9; i++ {
		got, _ := s.Pick(cs)
		seen[got.ID]++
	}
	if seen["a"] != 3 || seen["b"] != 3 || seen["c"] != 3 {
		t.Fatalf("expected 3 each, got %v", seen)
	}
}

func TestRandomPicksFromCandidates(t *testing.T) {
	s := NewSelector("random")
	cs := cands()
	valid := map[string]bool{"a": true, "b": true, "c": true}
	for i := 0; i < 50; i++ {
		got, ok := s.Pick(cs)
		if !ok || !valid[got.ID] {
			t.Fatalf("invalid pick %+v", got)
		}
	}
	// over 50 draws, expect >1 distinct (statistically near-certain)
	distinct := map[string]bool{}
	for i := 0; i < 50; i++ {
		got, _ := s.Pick(cs)
		distinct[got.ID] = true
	}
	if len(distinct) < 2 {
		t.Fatalf("random selector appears stuck on one option: %v", distinct)
	}
}

func TestTransportForEmptyIsDefault(t *testing.T) {
	if TransportFor("") != http.DefaultTransport {
		t.Fatal("empty proxy URL should return DefaultTransport")
	}
}

func TestEmbedCredentials(t *testing.T) {
	cases := []struct {
		name, url, user, pass, want string
	}{
		{"no creds", "http://1.2.3.4:8888", "", "", "http://1.2.3.4:8888"},
		{"user only", "socks5://h:1080", "bob", "", "socks5://bob@h:1080"},
		{"user+pass", "http://1.2.3.4:8888", "bob", "p@ss:word", "http://bob:p%40ss%3Aword@1.2.3.4:8888"},
		{"invalid url passthrough", "://bad", "u", "p", "://bad"},
	}
	for _, c := range cases {
		got := EmbedCredentials(c.url, c.user, c.pass)
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
