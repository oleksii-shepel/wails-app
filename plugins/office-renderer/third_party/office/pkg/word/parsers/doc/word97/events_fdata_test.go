package word97

import "testing"

func TestValidateRunDoesNotClassifyGenericFDataAsPicture(t *testing.T) {
	run := Run{Text: "form-data", Spec: true, Data: true}
	if err := validateRun(run); err != nil {
		t.Fatalf("validateRun(generic fData) = %v, want nil", err)
	}
}
