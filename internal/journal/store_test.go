package journal

import (
    "encoding/json"
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

func TestRecordsReturnsIndependentPayloads(t *testing.T) {
    directory := t.TempDir()
    store, err := NewStore(directory)
    if err != nil {
        t.Fatal(err)
    }
    evidence := model.NewEvidence("run-a", "rev-1", "left", model.EvidenceWaveTime, 0.5, time.Now().UTC())
    record, err := EvidenceRecord(evidence, time.Now().UTC())
    if err != nil {
        t.Fatal(err)
    }
    if err := store.Append(record); err != nil {
        t.Fatal(err)
    }
    original := string(store.Records()[0].Payload)

    returned := store.Records()
    for i := range returned[0].Payload {
        returned[0].Payload[i] = 'x'
    }

    if got := string(store.Records()[0].Payload); got != original {
        t.Fatalf("mutating returned payload rewrote in-memory history: got %s want %s", got, original)
    }
    reloaded, err := NewStore(directory)
    if err != nil {
        t.Fatal(err)
    }
    if got := string(reloaded.Records()[0].Payload); got != original {
        t.Fatalf("reloaded record diverged from original: got %s want %s", got, original)
    }
    var decoded model.Evidence
    if err := json.Unmarshal(store.Records()[0].Payload, &decoded); err != nil {
        t.Fatalf("stored payload corrupted: %v", err)
    }
    if decoded.Value != evidence.Value || decoded.SourceID != evidence.SourceID {
        t.Fatalf("stored evidence changed: %+v", decoded)
    }
}

func TestAppendDetachesCallerPayload(t *testing.T) {
    directory := t.TempDir()
    store, err := NewStore(directory)
    if err != nil {
        t.Fatal(err)
    }
    record, err := NewRecord("run-b", "fence", uint64(7), time.Now().UTC())
    if err != nil {
        t.Fatal(err)
    }
    original := string(record.Payload)
    if err := store.Append(record); err != nil {
        t.Fatal(err)
    }
    for i := range record.Payload {
        record.Payload[i] = '0'
    }
    if got := string(store.Records()[0].Payload); got != original {
        t.Fatalf("mutating caller payload after append rewrote stored record: got %s want %s", got, original)
    }
    reloaded, err := NewStore(directory)
    if err != nil {
        t.Fatal(err)
    }
    if got := string(reloaded.Records()[0].Payload); got != original {
        t.Fatalf("reloaded record diverged from original: got %s want %s", got, original)
    }
}
