package main

import (
	"github.com/spf13/viper"
)

type rowItem struct {
	path     string //
	branch   string //
	detached bool   //
	added    int    // New files
	modified int    // Modified files
	deleted  int    // Deleted files
	updated  bool   // If something was pulled down
	skipped  string // Why the pull was skipped
	error    error  //
}

func (r rowItem) show() bool {
	return viper.GetBool(fAll) || !r.isMain() || r.isDirty() || r.updated || r.skipped != "" || (r.error != nil)
}

func (r rowItem) isMain() bool {
	return r.branch == "master" || r.branch == "main" || r.branch == "trunk" || r.branch == "develop" || r.branch == "dev"
}

func (r rowItem) isDetached() bool {
	return r.detached
}

func (r rowItem) isDirty() bool {
	return r.added+r.modified+r.deleted > 0
}
