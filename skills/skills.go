// Package skills embeds the agent skills that ship with routerctl.
package skills

import (
	"embed"
	"io/fs"
	"sort"
)

// Name is the skill's name and the directory it installs into.
const Name = "router-tools"

//go:embed router-tools
var files embed.FS

// RouterTools returns the skill's files, rooted at the skill directory.
func RouterTools() fs.FS {
	sub, err := fs.Sub(files, Name)
	if err != nil {
		panic(err)
	}
	return sub
}

// Files lists the skill's files relative to its directory, SKILL.md first.
func Files() ([]string, error) {
	var names []string
	err := fs.WalkDir(RouterTools(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "SKILL.md" || names[j] == "SKILL.md" {
			return names[i] == "SKILL.md"
		}
		return names[i] < names[j]
	})
	return names, err
}
