// Package capability describes operations independently of their implementation.
package capability

type Descriptor struct {
	ID        string
	Operation string
	Sensitive bool
}

type Call struct {
	ID         string
	Collection string
	RecordID   string
	Values     map[string]any
	Approved   bool
}

type Record struct {
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

type Catalog []Descriptor

func (c Catalog) Lookup(id string) (Descriptor, bool) {
	for _, d := range c {
		if d.ID == id {
			return d, true
		}
	}
	return Descriptor{}, false
}
func Local() Catalog {
	return Catalog{
		{ID: "local.records.list", Operation: "list"},
		{ID: "local.records.insert", Operation: "insert"},
		{ID: "local.records.update", Operation: "update"},
		{ID: "local.records.delete", Operation: "delete", Sensitive: true},
	}
}
