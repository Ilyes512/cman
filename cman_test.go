package cman_test

import (
	"testing"

	"github.com/Ilyes512/cman"
)

func TestNewCm(t *testing.T) {
	cm := &cman.CMan{}
	if cm.Alive() == false {
		t.Error("cman.Alive() call should return true")
	}

	if cm.Err() != cman.ErrStillAlive {
		t.Error("cman.Err() call should return cman")
	}
}

func TestKillWithNill(t *testing.T) {
	cm := &cman.CMan{}
	cm.Kill(nil)
	if cm.Err() != nil {
		t.Error("cman.Err() call should return nil")
	}
}
