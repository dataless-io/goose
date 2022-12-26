package main

import (
	"testing"

	"github.com/fulldump/biff"
)

// TODO: move this to a proper package
func TestFindMentions(t *testing.T) {
	mentions := findMentions("Hello @Fulanez, here is the thing. cc @Menganez, @FULANEZ")

	biff.AssertEqual(mentions, []string{"fulanez", "menganez"})
}
