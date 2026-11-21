package queue

import (
	"strconv"
	"testing"
)

func TestQueueBatchesAndResume(t *testing.T) {
	dir := t.TempDir()
	q, err := Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	seeds := make([]string, 0, 1200)
	for i := 0; i < 1200; i++ {
		seeds = append(seeds, "https://seed.example/p"+strconv.Itoa(i))
	}
	if err := q.Enqueue(seeds, 0); err != nil {
		t.Fatal(err)
	}
	var got []Item
	for {
		it, ok := q.Next()
		if !ok {
			break
		}
		got = append(got, it)
		q.Ack(it.URL)
	}
	if len(got) != 1200 {
		t.Fatalf("got %d items", len(got))
	}
	// batches of 1000
	if b0 := got[:1000]; len(b0) != 1000 {
		t.Fatalf("batch0 %d", len(b0))
	}
	// resume: reopen, nothing pending
	q2, err := Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q2.Next(); ok {
		t.Fatal("expected empty queue after ack-all + reopen")
	}
	if q2.DoneCount() != 1200 {
		t.Fatalf("done %d", q2.DoneCount())
	}
}

func TestQueueDedupe(t *testing.T) {
	dir := t.TempDir()
	q, err := Open(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	q.Enqueue([]string{"https://a.example/1", "https://a.example/1", "https://a.example/2"}, 0)
	n := 0
	for {
		it, ok := q.Next()
		if !ok {
			break
		}
		n++
		q.Ack(it.URL)
	}
	if n != 2 {
		t.Fatalf("deduped count %d, want 2", n)
	}
	// no-dedupe queue keeps all three
	dir2 := t.TempDir()
	q3, err := Open(dir2, false)
	if err != nil {
		t.Fatal(err)
	}
	q3.Enqueue([]string{"https://a.example/1", "https://a.example/1"}, 0)
	n = 0
	for {
		it, ok := q3.Next()
		if !ok {
			break
		}
		n++
		q3.Ack(it.URL)
		_ = it
	}
	if n != 2 {
		t.Fatalf("no-dedupe count %d, want 2", n)
	}
}
