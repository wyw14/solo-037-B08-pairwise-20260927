package journal

import (
    "bufio"
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "sync"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

type Record struct {
    ID model.EventID `json:"id"`
    Run model.RunID `json:"run"`
    Kind string `json:"kind"`
    Payload json.RawMessage `json:"payload"`
    At time.Time `json:"at"`
}

type Store struct {
    mu sync.Mutex
    directory string
    failKind string
    records []Record
}

func NewStore(directory string) (*Store, error) {
    if err := os.MkdirAll(directory, 0755); err != nil {
        return nil, err
    }
    store := &Store{directory: directory}
    if err := store.load(); err != nil {
        return nil, err
    }
    return store, nil
}

func (s *Store) path() string {
    return filepath.Join(s.directory, "events.jsonl")
}

func (s *Store) load() error {
    file, err := os.Open(s.path())
    if errors.Is(err, os.ErrNotExist) {
        return nil
    }
    if err != nil {
        return err
    }
    defer file.Close()
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        var record Record
        if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
            return err
        }
        s.records = append(s.records, record)
    }
    return scanner.Err()
}

func (s *Store) FailNext(kind string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.failKind = kind
}

func (s *Store) Append(record Record) error {
    return s.AppendBatch([]Record{record})
}

func (s *Store) AppendBatch(records []Record) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    for _, record := range records {
        if record.Kind == s.failKind {
            s.failKind = ""
            return errors.New("incomplete write")
        }
    }
    pending := make([]Record, 0, len(s.records)+len(records))
    pending = append(pending, s.records...)
    for _, record := range records {
        // Copy at ingress so later mutation of the caller's payload slice
        // (including its backing array) cannot reach stored history.
        pending = append(pending, record.clone())
    }
    temporary := s.path() + ".pending"
    file, err := os.Create(temporary)
    if err != nil {
        return err
    }
    encoder := json.NewEncoder(file)
    for _, record := range pending {
        if err := encoder.Encode(record); err != nil {
            file.Close()
            return err
        }
    }
    if err := file.Sync(); err != nil {
        file.Close()
        return err
    }
    if err := file.Close(); err != nil {
        return err
    }
    if err := os.Rename(temporary, s.path()); err != nil {
        return err
    }
    s.records = pending
    return nil
}

func (s *Store) Records() []Record {
    s.mu.Lock()
    defer s.mu.Unlock()
    result := make([]Record, len(s.records))
    for i, record := range s.records {
        // Deep copy the payload: a shallow copy would let callers rewrite
        // history through the shared json.RawMessage backing array even though
        // the durable file on disk still holds the original bytes.
        result[i] = record.clone()
    }
    return result
}

// clone returns a record whose payload owns a fresh byte slice, so the result
// never shares mutable storage with any other record.
func (r Record) clone() Record {
    if r.Payload != nil {
        r.Payload = append(json.RawMessage{}, r.Payload...)
    }
    return r
}
