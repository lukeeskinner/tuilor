package codeapp

import _ "embed"

//go:embed ui.go.txt
var uiSource string

// UISource is also the generator's exact helper reference. Keep version 1 stable;
// a future helper contract needs a new version, not silent saved-app upgrades.
func UISource() string { return uiSource }

func (p *Project) writeUI() error {
	if p.Spec.UIVersion != 1 {
		return nil
	}
	return atomicFile(p.Dir+"/ui.go", []byte(uiSource), 0600)
}
