package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunTUICommand_Interactive(t *testing.T) {
	origRunTUI := runTUI
	origRunHeadlessUI := runHeadlessUI
	origHeadless := tuiHeadless
	t.Cleanup(func() {
		runTUI = origRunTUI
		runHeadlessUI = origRunHeadlessUI
		tuiHeadless = origHeadless
	})

	calledInteractive := 0
	calledHeadless := 0
	runTUI = func() error {
		calledInteractive++
		return nil
	}
	runHeadlessUI = func() error {
		calledHeadless++
		return nil
	}

	tuiHeadless = false
	err := runTUICommand(tuiCmd, []string{})
	assert.NoError(t, err)
	assert.Equal(t, 1, calledInteractive, "interactive runner should be called once")
	assert.Equal(t, 0, calledHeadless, "headless runner should not be called")
}

func TestRunTUICommand_Headless(t *testing.T) {
	origRunTUI := runTUI
	origRunHeadlessUI := runHeadlessUI
	origHeadless := tuiHeadless
	t.Cleanup(func() {
		runTUI = origRunTUI
		runHeadlessUI = origRunHeadlessUI
		tuiHeadless = origHeadless
	})

	calledInteractive := 0
	calledHeadless := 0
	runTUI = func() error {
		calledInteractive++
		return nil
	}
	runHeadlessUI = func() error {
		calledHeadless++
		return nil
	}

	tuiHeadless = true
	err := runTUICommand(tuiCmd, []string{})
	assert.NoError(t, err)
	assert.Equal(t, 0, calledInteractive, "interactive runner should not be called")
	assert.Equal(t, 1, calledHeadless, "headless runner should be called once")
}

func TestRunTUICommand_ErrorPropagation(t *testing.T) {
	origRunTUI := runTUI
	origRunHeadlessUI := runHeadlessUI
	origHeadless := tuiHeadless
	t.Cleanup(func() {
		runTUI = origRunTUI
		runHeadlessUI = origRunHeadlessUI
		tuiHeadless = origHeadless
	})

	interactiveErr := errors.New("interactive failure")
	runTUI = func() error { return interactiveErr }
	runHeadlessUI = func() error { return nil }

	tuiHeadless = false
	err := runTUICommand(tuiCmd, []string{})
	assert.ErrorIs(t, err, interactiveErr)

	headlessErr := errors.New("headless failure")
	runHeadlessUI = func() error { return headlessErr }
	runTUI = func() error { return nil }

	tuiHeadless = true
	err = runTUICommand(tuiCmd, []string{})
	assert.ErrorIs(t, err, headlessErr)
}
