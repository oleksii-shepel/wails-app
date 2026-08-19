package app

import "testing"

func TestBeforeCloseChoiceAllowsConfirmedShutdown(t *testing.T) {
	if beforeCloseChoicePreventsShutdown("Yes") {
		t.Fatal("affirmative QuestionDialog response must allow shutdown")
	}
}

func TestBeforeCloseChoicePreventsNegativeShutdown(t *testing.T) {
	if !beforeCloseChoicePreventsShutdown("No") {
		t.Fatal("negative QuestionDialog response must prevent shutdown")
	}
}

func TestBeforeCloseChoicePreventsDismissedDialog(t *testing.T) {
	if !beforeCloseChoicePreventsShutdown("") {
		t.Fatal("dismissed confirmation must prevent shutdown")
	}
}
