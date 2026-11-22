package queue

import "testing"

func TestInFlightRequeuedOnReopen(t *testing.T) {
	dir := t.TempDir()
	q, err := Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	q.Enqueue([]string{"https://a.example/1", "https://a.example/2"}, 0)
	it, ok := q.Next()
	if !ok {
		t.Fatal("expected item")
	}
	// crash: no Ack/Dead. Reopen must restore the in-flight item.
	q2, err := Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	it2, ok := q2.Next()
	if !ok {
		t.Fatal("in-flight item lost after reopen")
	}
	if it2.URL != it.URL {
		t.Fatalf("got %q want %q", it2.URL, it.URL)
	}
	q2.Ack(it2.URL)
	it3, ok := q2.Next()
	if !ok || it3.URL != "https://a.example/2" {
		t.Fatalf("second item: %+v %v", it3, ok)
	}
}
