package main

import (
	"time"
)

type rowItem struct {
	path         string     //
	branch       string     //
	changedFiles string     // Modified files
	updated      bool       // If something was pulled down
	lastCommit   *time.Time //
	error        error      //
}

func (r rowItem) show() bool {
	return *flagShowAll || !r.isMain() || r.isDirty() || r.updated || (r.error != nil)
}

func (r rowItem) isMain() bool {
	return r.branch == "master" || r.branch == "main"
}

func (r rowItem) isDirty() bool {
	return r.changedFiles != ""
}

func (r rowItem) daysStale() int {
	if r.lastCommit == nil {
		return 0
	}
	d := time.Since(*r.lastCommit)
	return int(d.Hours() / 24)
}

func (r rowItem) isStale() bool {
	return r.daysStale() > 180
}
