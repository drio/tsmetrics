package main

import (
	"testing"

	qt "github.com/frankban/quicktest"
	tscg "github.com/tailscale/tailscale-client-go/tailscale"
)

var fakeClient tscg.Client

//func (c *tscg.Clien)

func TestXXXXX(t *testing.T) {
	t.Run("numbers", func(t *testing.T) {
		//fClient := tscg.Client{}
		c := qt.New(t)
		var err error
		numbers := []int{12, 12}
		c.Assert(err, qt.IsNil)
		c.Assert(numbers, qt.DeepEquals, []int{12, 12})
	})
}
