package main

import (
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestConfigureRepoOptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := newMockRESTClient(ctrl)

	err := configureRepoOptions("my-repo", "myorg", "develop", mock)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mock.patchCalls) != 1 {
		t.Fatalf("expected 1 PATCH call, got %d", len(mock.patchCalls))
	}
	if mock.patchCalls[0].path != "repos/myorg/my-repo" {
		t.Errorf("unexpected PATCH path: %s", mock.patchCalls[0].path)
	}
	if !strings.Contains(mock.patchCalls[0].body, "allow_auto_merge") {
		t.Errorf("expected allow_auto_merge in body: %s", mock.patchCalls[0].body)
	}
}

func TestConfigureBranchProtection_Org(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := newMockRESTClient(ctrl)
	// GET orgs/ succeeds → it's a real org.
	mock.getErr = nil

	err := configureBranchProtection("my-repo", "myorg", "develop", mock)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have called GET orgs/myorg.
	if len(mock.getCalls) == 0 || mock.getCalls[0] != "orgs/myorg" {
		t.Errorf("expected GET orgs/myorg, got %v", mock.getCalls)
	}

	// Should have called PUT with dismissal_restrictions (org protection).
	if len(mock.putCalls) != 1 {
		t.Fatalf("expected 1 PUT call, got %d", len(mock.putCalls))
	}
	if !strings.Contains(mock.putCalls[0].body, "dismissal_restrictions") {
		t.Errorf("expected dismissal_restrictions in org branch protection body: %s", mock.putCalls[0].body)
	}
}

func TestConfigureBranchProtection_User(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := newMockRESTClient(ctrl)
	// GET orgs/ fails → it's a personal account.
	mock.getErr = fmt.Errorf("not found")

	err := configureBranchProtection("my-repo", "myuser", "develop", mock)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have called GET orgs/myuser.
	if len(mock.getCalls) == 0 || mock.getCalls[0] != "orgs/myuser" {
		t.Errorf("expected GET orgs/myuser, got %v", mock.getCalls)
	}

	// Should have called PUT without dismissal_restrictions (user protection).
	if len(mock.putCalls) != 1 {
		t.Fatalf("expected 1 PUT call, got %d", len(mock.putCalls))
	}
	if strings.Contains(mock.putCalls[0].body, "dismissal_restrictions") {
		t.Errorf("did not expect dismissal_restrictions in user branch protection body: %s", mock.putCalls[0].body)
	}
}
