package journal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wyw14/solo-037/internal/model"
)

func TestRecordsPayloadsAreIndependentCopies(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	record, err := NewRecord(model.RunID("run-1"), "fence", uint64(7), time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	original := append(json.RawMessage{}, record.Payload...)

	// Mutate the caller's payload after appending: stored history must be unaffected.
	if err := store.Append(record); err != nil {
		t.Fatal(err)
	}
	for i := range record.Payload {
		record.Payload[i] = 'X'
	}

	// Mutate the payload handed back by a query: in-memory history must be unaffected.
	queried := store.Records()
	if len(queried) != 1 {
		t.Fatalf("expected 1 record, got %d", len(queried))
	}
	for i := range queried[0].Payload {
		queried[0].Payload[i] = 'Z'
	}

	again := store.Records()
	if string(again[0].Payload) != string(original) {
		t.Fatalf("in-memory history was altered through query result: got %q, want %q",
			again[0].Payload, original)
	}

	// Records handed out on separate reads must not share storage either.
	if &again[0].Payload[0] == &store.Records()[0].Payload[0] {
		t.Fatal("repeated reads return payloads sharing backing arrays")
	}

	// After reopening, the durable original must match the in-memory record.
	reopened, err := NewStore(store.directory)
	if err != nil {
		t.Fatal(err)
	}
	loaded := reopened.Records()
	if string(loaded[0].Payload) != string(original) {
		t.Fatalf("durable record diverged from in-memory history: got %q, want %q",
			loaded[0].Payload, original)
	}

	// The loaded copy is independent as well.
	for i := range loaded[0].Payload {
		loaded[0].Payload[i] = 'Y'
	}
	if string(reopened.Records()[0].Payload) != string(original) {
		t.Fatal("mutating a record loaded from disk altered store history")
	}
}

func TestAppendBatchDefensiveCopy(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := NewRecord(model.RunID("run-1"), "fence", uint64(1), time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRecord(model.RunID("run-1"), "fence", uint64(2), time.Unix(1700000001, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendBatch([]Record{first, second}); err != nil {
		t.Fatal(err)
	}
	for i := range first.Payload {
		first.Payload[i] = 'Q'
	}
	records := store.Records()
	if string(records[0].Payload) == string(first.Payload) {
		t.Fatal("AppendBatch stored an alias of the caller's batch payload")
	}
}
