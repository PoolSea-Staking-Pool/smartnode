package config

import (
	"fmt"

	"github.com/rocket-pool/smartnode/shared"
)

func createWelcomeStep(wiz *wizard, currentStep int, totalSteps int) *choiceWizardStep {

	var intro string
	if wiz.md.isMigration {
		intro = "It looks like you're migrating from a previous version of the Smartnode.\nWe have done our best to capture all of your previous settings, but please confirm them using this wizard and the settings management screen after it."
	} else if wiz.md.isNew {
		intro = "Since this is your first time configuring the Smartnode, we'll walk you through the basic setup.\n\n"
	} else {
		intro = "You've already configured poolsea Pool, so we'll highlight all of the settings you're already using for convenience. You're welcome to make changes as you go through the wizard."
	}

	helperText := fmt.Sprintf("%s\n\nWelcome to the Smartnode configuration wizard!\n\n%s\n\n", shared.Logo, intro)

	show := func(modal *choiceModalLayout) {
		wiz.md.setPage(modal.page)
		modal.focus(1)
	}

	done := func(buttonIndex int, buttonLabel string) {
		if buttonIndex == 1 {
			wiz.networkModal.show()
		} else {
			wiz.md.app.Stop()
		}
	}

	back := func() {
	}

	return newChoiceStep(
		wiz,
		currentStep,
		totalSteps,
		helperText,
		[]string{"Quit", "Next"},
		nil,
		60,
		"Welcome",
		DirectionalModalHorizontal,
		show,
		done,
		back,
		"step-welcome",
	)

}
