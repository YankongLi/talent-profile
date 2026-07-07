package main

import (
	"errors"
	"testing"
)

type fakeMigrator struct {
	upCalled     bool
	steps        int
	version      uint
	dirty        bool
	versionError error
}

func (f *fakeMigrator) Up() error {
	f.upCalled = true
	return nil
}

func (f *fakeMigrator) Steps(n int) error {
	f.steps = n
	return nil
}

func (f *fakeMigrator) Version() (uint, bool, error) {
	return f.version, f.dirty, f.versionError
}

func TestRunUp(t *testing.T) {
	migrator := &fakeMigrator{}

	if err := run([]string{"up"}, migrator); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !migrator.upCalled {
		t.Fatal("Up() was not called")
	}
}

func TestRunDownDefaultsToOneStep(t *testing.T) {
	migrator := &fakeMigrator{}

	if err := run([]string{"down"}, migrator); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if migrator.steps != -1 {
		t.Fatalf("Steps() = %d, want -1", migrator.steps)
	}
}

func TestRunDownUsesProvidedSteps(t *testing.T) {
	migrator := &fakeMigrator{}

	if err := run([]string{"down", "3"}, migrator); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if migrator.steps != -3 {
		t.Fatalf("Steps() = %d, want -3", migrator.steps)
	}
}

func TestRunDownRejectsInvalidSteps(t *testing.T) {
	migrator := &fakeMigrator{}

	if err := run([]string{"down", "0"}, migrator); err == nil {
		t.Fatal("run() error = nil, want error")
	}
}

func TestRunVersionPropagatesErrors(t *testing.T) {
	wantErr := errors.New("version failed")
	migrator := &fakeMigrator{versionError: wantErr}

	err := run([]string{"version"}, migrator)
	if !errors.Is(err, wantErr) {
		t.Fatalf("run() error = %v, want %v", err, wantErr)
	}
}
