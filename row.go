package main

import (
	"github.com/spf13/viper"
)

type rowItem struct {
	path         string //
	branch       string //
	changedFiles string // Modified files
	updated      bool   // If something was pulled down
	error        error  //
}

func (r rowItem) show() bool {
	return viper.GetBool(fAll) || !r.isMain() || r.isDirty() || r.updated || (r.error != nil)
}

func (r rowItem) isMain() bool {
	return r.branch == "master" || r.branch == "main"
}

func (r rowItem) isDirty() bool {
	return r.changedFiles != ""
}
