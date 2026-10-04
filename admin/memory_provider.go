package admin

import (
	"fmt"
	"maps"
	"sync"
)

// MemoryProvider is a generic, thread-safe, in-memory DataProvider. It lets a
// Resource be wired up with zero storage code; swap it for a database-backed
// DataProvider implementation when persistence is needed.
type MemoryProvider struct {
	mu     sync.Mutex
	nextID int
	items  []Record
}

// NewMemoryProvider creates a MemoryProvider seeded with the given records.
// Seed records do not need an "id" — one is assigned automatically.
func NewMemoryProvider(seed []Record) *MemoryProvider {
	p := &MemoryProvider{nextID: 1}
	for _, rec := range seed {
		cp := cloneRecord(rec)
		cp["id"] = fmt.Sprintf("%d", p.nextID)
		p.nextID++
		p.items = append(p.items, cp)
	}
	return p
}

func cloneRecord(r Record) Record {
	out := make(Record, len(r))
	maps.Copy(out, r)
	return out
}

func (p *MemoryProvider) List(page, pageSize int) (ListResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := len(p.items)
	if page < 1 {
		page = 1
	}
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)

	out := make([]Record, 0, end-start)
	for _, rec := range p.items[start:end] {
		out = append(out, cloneRecord(rec))
	}
	return ListResult{Records: out, Total: total}, nil
}

func (p *MemoryProvider) Get(id string) (Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, rec := range p.items {
		if rec.ID() == id {
			return cloneRecord(rec), nil
		}
	}
	return nil, fmt.Errorf("record %q not found", id)
}

func (p *MemoryProvider) Create(data Record) (Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	rec := cloneRecord(data)
	rec["id"] = fmt.Sprintf("%d", p.nextID)
	p.nextID++
	p.items = append(p.items, rec)
	return cloneRecord(rec), nil
}

func (p *MemoryProvider) Update(id string, data Record) (Record, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, rec := range p.items {
		if rec.ID() == id {
			updated := cloneRecord(data)
			updated["id"] = id
			p.items[i] = updated
			return cloneRecord(updated), nil
		}
	}
	return nil, fmt.Errorf("record %q not found", id)
}

func (p *MemoryProvider) Delete(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, rec := range p.items {
		if rec.ID() == id {
			p.items = append(p.items[:i], p.items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("record %q not found", id)
}
